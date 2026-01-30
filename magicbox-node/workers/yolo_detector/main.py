#!/usr/bin/env python3
"""
YOLOv8 Object Detection Worker for MagicBox.

Detects objects in camera frames using YOLOv8 nano model.
- Publishes bounding boxes to NATS for UI overlay
- Sends alerts to platform when persons are detected
"""

import asyncio
import json
import time
import os
import sys
import logging
from typing import List, Optional
from dataclasses import dataclass

import numpy as np

# Add parent directory to path for base worker import
sys.path.append(os.path.join(os.path.dirname(__file__), '..', 'base'))
from worker import BaseWorker, FrameData, Event, decode_frame_to_numpy

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(name)s: %(message)s'
)


@dataclass
class Detection:
    """Single object detection"""
    class_id: int
    class_name: str
    confidence: float
    x: int  # Top-left x
    y: int  # Top-left y
    width: int
    height: int
    
    def to_dict(self) -> dict:
        return {
            "type": self.class_name,
            "confidence": round(self.confidence, 3),
            "bbox": [self.x, self.y, self.width, self.height],
            "label": f"{self.class_name} {int(self.confidence * 100)}%",
        }


class YOLODetectorWorker(BaseWorker):
    """
    YOLO-based object detector worker.
    
    Processes frames with YOLOv8 and:
    - Publishes all detections to NATS for UI overlay
    - Sends alerts for person detections
    """
    
    # COCO class names
    COCO_CLASSES = [
        'person', 'bicycle', 'car', 'motorcycle', 'airplane', 'bus', 'train', 'truck',
        'boat', 'traffic light', 'fire hydrant', 'stop sign', 'parking meter', 'bench',
        'bird', 'cat', 'dog', 'horse', 'sheep', 'cow', 'elephant', 'bear', 'zebra',
        'giraffe', 'backpack', 'umbrella', 'handbag', 'tie', 'suitcase', 'frisbee',
        'skis', 'snowboard', 'sports ball', 'kite', 'baseball bat', 'baseball glove',
        'skateboard', 'surfboard', 'tennis racket', 'bottle', 'wine glass', 'cup',
        'fork', 'knife', 'spoon', 'bowl', 'banana', 'apple', 'sandwich', 'orange',
        'broccoli', 'carrot', 'hot dog', 'pizza', 'donut', 'cake', 'chair', 'couch',
        'potted plant', 'bed', 'dining table', 'toilet', 'tv', 'laptop', 'mouse',
        'remote', 'keyboard', 'cell phone', 'microwave', 'oven', 'toaster', 'sink',
        'refrigerator', 'book', 'clock', 'vase', 'scissors', 'teddy bear', 'hair drier',
        'toothbrush'
    ]
    
    # Classes to generate alerts for
    ALERT_CLASSES = ['person']
    
    def __init__(
        self,
        cameras: List[str],
        confidence_threshold: float = 0.5,
        alert_cooldown: float = 5.0,  # Seconds between alerts per camera
        **kwargs
    ):
        super().__init__(
            worker_type="yolo_detector",
            cameras=cameras,
            **kwargs
        )
        
        self.confidence_threshold = confidence_threshold
        self.alert_cooldown = alert_cooldown
        self.model = None
        
        # Track last alert time per camera to avoid spam
        self.last_alert_time: dict[str, float] = {}
        
        # FPS tracking
        self.fps_count = 0
        self.fps_last_time = time.time()
        self.current_fps = 0.0
    
    async def on_start(self) -> None:
        """Load YOLO model on startup"""
        self.logger.info("Loading YOLOv8 nano model...")
        
        try:
            from ultralytics import YOLO
            
            # Load YOLOv8 nano - will auto-download if not present
            self.model = YOLO('yolov8n.pt')
            
            # Warm up the model with a dummy inference
            dummy = np.zeros((640, 640, 3), dtype=np.uint8)
            self.model.predict(dummy, verbose=False)
            
            self.logger.info("✅ YOLOv8 nano model loaded successfully")
            
        except ImportError:
            self.logger.error("❌ ultralytics not installed. Run: pip install ultralytics")
            raise
        except Exception as e:
            self.logger.error(f"❌ Failed to load YOLO model: {e}")
            raise
    
    async def process_frame(self, frame: FrameData) -> List[Event]:
        """Process frame with YOLO and return events"""
        events = []
        
        # Decode JPEG to numpy array
        img = decode_frame_to_numpy(frame)
        if img is None:
            self.logger.warning(f"Failed to decode frame from {frame.camera_id}")
            return events
        
        # Run YOLO inference
        try:
            results = self.model.predict(
                img,
                conf=self.confidence_threshold,
                verbose=False,
                device='cpu'  # Use 'cuda' if GPU available
            )
        except Exception as e:
            self.logger.error(f"YOLO inference failed: {e}")
            return events
        
        # Parse detections
        detections: List[Detection] = []
        person_count = 0
        
        if results and len(results) > 0:
            result = results[0]
            
            if result.boxes is not None:
                for box in result.boxes:
                    class_id = int(box.cls[0])
                    confidence = float(box.conf[0])
                    
                    # Get bounding box (xyxy format)
                    x1, y1, x2, y2 = box.xyxy[0].tolist()
                    
                    class_name = self.COCO_CLASSES[class_id] if class_id < len(self.COCO_CLASSES) else f"class_{class_id}"
                    
                    detection = Detection(
                        class_id=class_id,
                        class_name=class_name,
                        confidence=confidence,
                        x=int(x1),
                        y=int(y1),
                        width=int(x2 - x1),
                        height=int(y2 - y1),
                    )
                    detections.append(detection)
                    
                    if class_name == 'person':
                        person_count += 1
        
        # Publish detections to NATS for UI overlay
        if detections:
            await self._publish_detections(frame.camera_id, detections)
        
        # Generate person alert if cooldown has passed
        if person_count > 0:
            now = time.time()
            last_alert = self.last_alert_time.get(frame.camera_id, 0)
            
            if now - last_alert >= self.alert_cooldown:
                self.last_alert_time[frame.camera_id] = now
                
                alert_event = Event(
                    event_type="person_detected",
                    camera_id=frame.camera_id,
                    data={
                        "count": person_count,
                        "detections": [d.to_dict() for d in detections if d.class_name == 'person'],
                        "total_objects": len(detections),
                    },
                    frame=frame.jpeg_bytes,  # Include frame snapshot
                )
                events.append(alert_event)
                
                self.logger.info(f"🚨 Person detected: {person_count} on {frame.camera_id}")
        
        # Update FPS tracking
        self.fps_count += 1
        now = time.time()
        if now - self.fps_last_time >= 1.0:
            self.current_fps = self.fps_count / (now - self.fps_last_time)
            self.logger.info(f"📊 [YOLO] {frame.camera_id}: {self.current_fps:.1f} fps, {len(detections)} detections")
            self.fps_count = 0
            self.fps_last_time = now
        
        return events
    
    async def _publish_detections(self, camera_id: str, detections: List[Detection]) -> None:
        """Publish detections to NATS for UI overlay"""
        if not self.nc or not self.nc.is_connected:
            return
        
        # Format for UI overlay
        detection_data = {
            "camera_id": camera_id,
            "timestamp": int(time.time() * 1000),
            "detections": [d.to_dict() for d in detections],
        }
        
        subject = f"detections.{camera_id}"
        
        try:
            await self.nc.publish(subject, json.dumps(detection_data).encode())
        except Exception as e:
            self.logger.error(f"Failed to publish detections: {e}")


def main():
    import argparse
    
    parser = argparse.ArgumentParser(description="YOLOv8 Object Detection Worker")
    parser.add_argument(
        '--cameras', '-c',
        type=str,
        default=os.environ.get('CAMERAS', ''),
        help='Comma-separated list of camera IDs to process'
    )
    parser.add_argument(
        '--nats-url',
        type=str,
        default=os.environ.get('NATS_URL', 'nats://localhost:4222'),
        help='NATS server URL'
    )
    parser.add_argument(
        '--platform-url',
        type=str,
        default=os.environ.get('PLATFORM_URL', 'http://localhost:3001'),
        help='Platform API URL for alerts'
    )
    parser.add_argument(
        '--confidence',
        type=float,
        default=float(os.environ.get('YOLO_CONFIDENCE', '0.5')),
        help='Minimum confidence threshold (0-1)'
    )
    parser.add_argument(
        '--alert-cooldown',
        type=float,
        default=float(os.environ.get('ALERT_COOLDOWN', '5.0')),
        help='Seconds between person alerts per camera'
    )
    
    args = parser.parse_args()
    
    # Parse cameras
    cameras = [c.strip() for c in args.cameras.split(',') if c.strip()]
    
    if not cameras:
        print("Error: No cameras specified. Use --cameras cam_001,cam_002 or set CAMERAS env var")
        print("Or leave empty to subscribe to all cameras (frames.*)")
        cameras = []  # Will subscribe to frames.*
    
    # Create and run worker
    worker = YOLODetectorWorker(
        cameras=cameras,
        nats_url=args.nats_url,
        platform_url=args.platform_url,
        confidence_threshold=args.confidence,
        alert_cooldown=args.alert_cooldown,
    )
    
    asyncio.run(worker.run())


if __name__ == "__main__":
    main()

