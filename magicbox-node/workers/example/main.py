#!/usr/bin/env python3
"""
Example MagicBox Worker

This worker demonstrates how to:
1. Subscribe to camera frames via NATS
2. Process frames (e.g., run inference)
3. Send events back to the IRIS platform

Usage:
    # Set environment variables
    export NATS_URL="nats://localhost:4222"
    export CAMERAS="cam_001,cam_002"
    export PLATFORM_URL="http://localhost:3001"
    
    # Run worker
    python main.py

Or with command line args:
    python main.py --nats nats://localhost:4222 --cameras cam_001,cam_002
"""

import sys
import os
import argparse
import asyncio
import time
from typing import List

# Add parent directory to path for imports
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from base.worker import BaseWorker, FrameData, Event, decode_frame_to_numpy


class ExampleWorker(BaseWorker):
    """
    Example worker that logs frame info and occasionally sends test events.
    
    Replace the process_frame method with your actual inference logic.
    """
    
    def __init__(self, **kwargs):
        super().__init__(worker_type="example", **kwargs)
        self.frame_count = 0
        self.last_log_time = time.time()
    
    async def on_start(self) -> None:
        """Called when worker starts"""
        self.logger.info("Example worker initializing...")
        # Load your ML model here
        # self.model = load_model("path/to/model")
    
    async def on_stop(self) -> None:
        """Called when worker stops"""
        self.logger.info("Example worker cleaning up...")
        # Release resources here
    
    async def process_frame(self, frame: FrameData) -> List[Event]:
        """
        Process a frame and detect events.
        
        This is where you would run your ML inference.
        """
        self.frame_count += 1
        events = []
        
        # Log frame info periodically (every 5 seconds)
        now = time.time()
        if now - self.last_log_time >= 5.0:
            self.logger.info(
                f"📷 Camera {frame.camera_id}: "
                f"frame #{frame.seq}, "
                f"{frame.width}x{frame.height}, "
                f"{len(frame.jpeg_bytes)} bytes, "
                f"fps: {self.frame_count / 5:.1f}"
            )
            self.frame_count = 0
            self.last_log_time = now
        
        # Example: Decode frame if you need pixel data
        # img = decode_frame_to_numpy(frame)
        # if img is not None:
        #     # Run your inference
        #     detections = self.model.predict(img)
        #     
        #     for det in detections:
        #         events.append(Event(
        #             event_type="object_detected",
        #             camera_id=frame.camera_id,
        #             data={
        #                 "class": det.class_name,
        #                 "confidence": det.confidence,
        #                 "bbox": det.bbox,
        #             },
        #             frame=frame.jpeg_bytes,  # Include frame with event
        #         ))
        
        # Example: Send a test event every 100 frames
        if frame.seq % 100 == 0:
            events.append(Event(
                event_type="test_event",
                camera_id=frame.camera_id,
                data={
                    "message": f"Test event from frame {frame.seq}",
                    "frame_size": len(frame.jpeg_bytes),
                },
                # frame=frame.jpeg_bytes,  # Uncomment to include frame
            ))
            self.logger.info(f"📤 Sending test event for frame {frame.seq}")
        
        return events


def parse_args():
    parser = argparse.ArgumentParser(description="Example MagicBox Worker")
    parser.add_argument(
        "--nats",
        default=os.environ.get("NATS_URL", "nats://localhost:4222"),
        help="NATS server URL"
    )
    parser.add_argument(
        "--cameras",
        default=os.environ.get("CAMERAS", ""),
        help="Comma-separated list of camera IDs to subscribe to"
    )
    parser.add_argument(
        "--platform",
        default=os.environ.get("PLATFORM_URL", "http://localhost:3001"),
        help="IRIS platform URL"
    )
    parser.add_argument(
        "--worker-id",
        default=os.environ.get("WORKER_ID"),
        help="Worker ID"
    )
    return parser.parse_args()


async def main():
    args = parse_args()
    
    cameras = [c.strip() for c in args.cameras.split(",") if c.strip()] if args.cameras else None
    
    worker = ExampleWorker(
        cameras=cameras,
        nats_url=args.nats,
        platform_url=args.platform,
        worker_id=args.worker_id,
    )
    
    await worker.run()


if __name__ == "__main__":
    asyncio.run(main())

