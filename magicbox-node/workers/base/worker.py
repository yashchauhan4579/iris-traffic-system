#!/usr/bin/env python3
"""
Base worker class for MagicBox analytics workers.

Workers subscribe to frames from NATS and process them.
Events are sent directly to the IRIS platform.
"""

import asyncio
import json
import base64
import time
import os
import logging
import signal
import sys
from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import Optional, List, Callable, Any

import nats
from nats.aio.client import Client as NATSClient
import aiohttp
import numpy as np

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(name)s: %(message)s'
)


@dataclass
class FrameData:
    """Decoded frame data from NATS message"""
    camera_id: str
    seq: int
    timestamp: int  # milliseconds
    width: int
    height: int
    jpeg_bytes: bytes
    
    @property
    def timestamp_sec(self) -> float:
        return self.timestamp / 1000.0


@dataclass
class Event:
    """Event to send to platform"""
    event_type: str
    camera_id: str
    data: dict
    frame: Optional[bytes] = None  # Optional JPEG frame to include
    timestamp: Optional[float] = None
    
    def __post_init__(self):
        if self.timestamp is None:
            self.timestamp = time.time()


class BaseWorker(ABC):
    """
    Base class for analytics workers.
    
    Subclass this and implement the `process_frame` method.
    
    Example:
        class MyWorker(BaseWorker):
            def __init__(self):
                super().__init__(
                    worker_type="my_analytics",
                    cameras=["cam_001", "cam_002"]
                )
            
            async def process_frame(self, frame: FrameData) -> List[Event]:
                # Your inference logic here
                return []
        
        if __name__ == "__main__":
            worker = MyWorker()
            asyncio.run(worker.run())
    """
    
    def __init__(
        self,
        worker_type: str,
        cameras: Optional[List[str]] = None,
        nats_url: Optional[str] = None,
        platform_url: Optional[str] = None,
        worker_id: Optional[str] = None,
    ):
        self.worker_type = worker_type
        self.cameras = cameras or os.environ.get('CAMERAS', '').split(',')
        self.cameras = [c.strip() for c in self.cameras if c.strip()]
        
        self.nats_url = nats_url or os.environ.get('NATS_URL', 'nats://localhost:4222')
        self.platform_url = platform_url or os.environ.get('PLATFORM_URL', 'http://localhost:3001')
        self.worker_id = worker_id or os.environ.get('WORKER_ID', f'{worker_type}_{os.getpid()}')
        
        self.nc: Optional[NATSClient] = None
        self.running = False
        self.frames_processed = 0
        self.events_sent = 0
        self.last_frame_time: Optional[float] = None
        
        self.logger = logging.getLogger(self.worker_type)
        
        # Setup signal handlers
        signal.signal(signal.SIGINT, self._signal_handler)
        signal.signal(signal.SIGTERM, self._signal_handler)
    
    def _signal_handler(self, signum, frame):
        self.logger.info("Shutdown signal received")
        self.running = False
    
    async def connect(self) -> None:
        """Connect to NATS server"""
        self.nc = await nats.connect(
            self.nats_url,
            name=self.worker_id,
            reconnect_time_wait=2,
            max_reconnect_attempts=-1,
        )
        self.logger.info(f"Connected to NATS: {self.nats_url}")
    
    async def disconnect(self) -> None:
        """Disconnect from NATS"""
        if self.nc:
            await self.nc.close()
            self.logger.info("Disconnected from NATS")
    
    async def send_event(self, event: Event) -> bool:
        """Send an event to the IRIS platform"""
        try:
            # Build event in backend's expected format
            event_payload = {
                "id": f"{self.worker_id}_{int(event.timestamp * 1000)}",
                "timestamp": time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(event.timestamp)),
                "worker_id": self.worker_id,
                "device_id": event.camera_id,
                "type": event.event_type,
                "data": event.data,
            }
            
            async with aiohttp.ClientSession() as session:
                if event.frame:
                    # Multipart form data with frame
                    data = aiohttp.FormData()
                    data.add_field('event', json.dumps(event_payload), content_type='application/json')
                    data.add_field(
                        'frame',
                        event.frame,
                        filename='frame.jpg',
                        content_type='image/jpeg'
                    )
                    
                    async with session.post(
                        f"{self.platform_url}/api/events/ingest",
                        data=data,
                        headers={'X-Worker-ID': self.worker_id}
                    ) as resp:
                        success = resp.status == 200
                        if not success:
                            body = await resp.text()
                            self.logger.warning(f"Failed to send event: {resp.status} - {body}")
                else:
                    # JSON batch format (backend expects {"events": [...]})
                    batch_payload = {"events": [event_payload]}
                    async with session.post(
                        f"{self.platform_url}/api/events/ingest",
                        json=batch_payload,
                        headers={
                            'X-Worker-ID': self.worker_id,
                            'Content-Type': 'application/json'
                        }
                    ) as resp:
                        success = resp.status == 200
                        if not success:
                            body = await resp.text()
                            self.logger.warning(f"Failed to send event: {resp.status} - {body}")
                
                if success:
                    self.events_sent += 1
                    self.logger.debug(f"Event sent: {event.event_type}")
                
                return success
                
        except Exception as e:
            self.logger.error(f"Error sending event: {e}")
            return False
    
    async def _handle_message(self, msg) -> None:
        """Handle incoming NATS message"""
        try:
            data = json.loads(msg.data)
            
            # Decode frame
            frame = FrameData(
                camera_id=data['c'],
                seq=data['s'],
                timestamp=data['t'],
                width=data['w'],
                height=data['h'],
                jpeg_bytes=base64.b64decode(data['f']),
            )
            
            self.frames_processed += 1
            self.last_frame_time = time.time()
            
            # Process frame (implemented by subclass)
            events = await self.process_frame(frame)
            
            # Send any generated events
            if events:
                for event in events:
                    await self.send_event(event)
                    
        except Exception as e:
            self.logger.error(f"Error processing message: {e}")
    
    @abstractmethod
    async def process_frame(self, frame: FrameData) -> List[Event]:
        """
        Process a frame and return any detected events.
        
        Override this method in your worker subclass.
        
        Args:
            frame: The decoded frame data
            
        Returns:
            List of events to send to platform (can be empty)
        """
        pass
    
    async def on_start(self) -> None:
        """Called when worker starts. Override for initialization."""
        pass
    
    async def on_stop(self) -> None:
        """Called when worker stops. Override for cleanup."""
        pass
    
    async def run(self) -> None:
        """Main run loop"""
        self.running = True
        
        self.logger.info(f"Starting {self.worker_type} worker")
        self.logger.info(f"  Worker ID: {self.worker_id}")
        self.logger.info(f"  NATS URL: {self.nats_url}")
        self.logger.info(f"  Platform URL: {self.platform_url}")
        self.logger.info(f"  Cameras: {', '.join(self.cameras)}")
        
        try:
            # Connect to NATS
            await self.connect()
            
            # Call startup hook
            await self.on_start()
            
            # Subscribe to camera frames
            subscriptions = []
            for camera in self.cameras:
                subject = f"frames.{camera}"
                sub = await self.nc.subscribe(subject, cb=self._handle_message)
                subscriptions.append(sub)
                self.logger.info(f"Subscribed to: {subject}")
            
            # Also subscribe to wildcard if no specific cameras
            if not self.cameras:
                sub = await self.nc.subscribe("frames.*", cb=self._handle_message)
                subscriptions.append(sub)
                self.logger.info("Subscribed to: frames.* (all cameras)")
            
            self.logger.info(f"🚀 {self.worker_type} worker running")
            
            # Keep running until stopped
            while self.running:
                await asyncio.sleep(1)
                
                # Log stats periodically
                if self.frames_processed > 0 and self.frames_processed % 100 == 0:
                    self.logger.info(
                        f"Stats: {self.frames_processed} frames processed, "
                        f"{self.events_sent} events sent"
                    )
            
            # Cleanup
            for sub in subscriptions:
                await sub.unsubscribe()
            
            await self.on_stop()
            await self.disconnect()
            
        except Exception as e:
            self.logger.error(f"Worker error: {e}")
            raise
        finally:
            self.logger.info(f"Worker stopped. Processed {self.frames_processed} frames, sent {self.events_sent} events")


def decode_frame_to_numpy(frame: FrameData) -> Optional[np.ndarray]:
    """
    Decode JPEG frame to numpy array (requires OpenCV).
    
    Returns:
        numpy array in BGR format, or None if decoding fails
    """
    try:
        import cv2
        nparr = np.frombuffer(frame.jpeg_bytes, np.uint8)
        img = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
        return img
    except ImportError:
        logging.warning("OpenCV not installed. Install with: pip install opencv-python")
        return None
    except Exception as e:
        logging.error(f"Failed to decode frame: {e}")
        return None


# Convenience function for simple workers
def run_worker(worker_class, **kwargs):
    """Run a worker class with asyncio"""
    worker = worker_class(**kwargs)
    asyncio.run(worker.run())

