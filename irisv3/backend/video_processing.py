import cv2
import os
import asyncio
import base64
import gemini_service
import crowd_analysis
from datetime import datetime
from fastapi import WebSocket
import logging
import json
from database import SessionLocal, CrowdInsight

logger = logging.getLogger(__name__)

async def stream_video_analysis(video_path: str, websocket: WebSocket):
    logger.info(f"Starting analysis stream for: {video_path}")
    cap = cv2.VideoCapture(video_path)
    fps = cap.get(cv2.CAP_PROP_FPS)
    if fps == 0: fps = 30
    
    logger.info(f"Video FPS: {fps}")

    frame_interval = int(fps) 
    frame_count = 0
    
    filename = os.path.basename(video_path)
    db = SessionLocal()
    
    # Buffer to store frame paths for multi-frame analysis
    # We want 3 frames. Since we process every ~1 sec, we can keep the last 3 processed frames 
    # OR we can grab 3 consecutive raw frames at the time of processing. 
    # Grabbing 3 raw frames is better for movement detection within a short window.
    
    try:
        while cap.isOpened():
            ret, frame = cap.read()
            if not ret:
                break
                
            if frame_count % frame_interval == 0:
                timestamp = datetime.now().isoformat()
                
                # 1. Capture 3 consecutive frames for analysis
                # Current frame is 'frame'
                frames_to_analyze = []
                frame_paths = []
                
                # Save current frame
                path0 = f"/tmp/temp_gemini_{os.getpid()}_0.jpg"
                cv2.imwrite(path0, frame)
                frames_to_analyze.append(frame) # Keep in memory if needed, but we use paths
                frame_paths.append(path0)
                
                # Try to read next 2 frames for context (movement)
                # Note: this advances the capture pointer, which is fine as we skip frames anyway
                # But we need to be careful not to lose sync if we depend on frame_count for timeline.
                # Actually, reading 2 more frames means we advance by 2. 
                # frame_count usually tracks *processed* frames or *video* frames? 
                # Here frame_count is tracking video loop iterations. 
                # If we read extra frames, we should increment a counter or just peek.
                # cv2 doesn't peek easily. 
                # Strategy: We are at frame N. Read N+1, N+2. Use them for analysis. 
                # Then continue loop. The loop will increment frame_count manually? 
                # The loop increments frame_count by 1 each iteration. 
                # If we read here, we consume frames.
                
                # Simpler approach for "3 consecutive frames":
                # Just buffer the last 3 frames processed? 
                # If we process at 1fps, 3 frames cover 3 seconds. That is good for "general movement".
                # If we want "micro movement", we need N, N+1, N+2.
                # Let's do N, N+5, N+10 (approx 200ms apart) for better motion sense?
                # Or just N, N+1, N+2.
                
                # Let's read 2 more frames now.
                for k in range(2):
                    r, f = cap.read()
                    if r:
                        p = f"/tmp/temp_gemini_{os.getpid()}_{k+1}.jpg"
                        cv2.imwrite(p, f)
                        frame_paths.append(p)
                    else:
                        break # End of video
                
                # 2. Analyze using the list of frames
                try:
                    logger.info(f"Analyzing frame {frame_count} (and next 2) with Gemini")
                    # Use the new analyze_frames method
                    insight = await gemini_service.analyze_frames(frame_paths)
                except Exception as e:
                    logger.error(f"Gemini error at frame {frame_count}: {e}", exc_info=True)
                    insight = {"error": str(e)}
                
                # Normalize field names so frontend gets consistent keys
                if "congestion_level" in insight and "congestion" not in insight:
                    insight["congestion"] = insight.get("congestion_level")
                if "free_space" in insight and isinstance(insight["free_space"], str):
                    # Try to coerce string percentages like "80" or "80%" into int
                    try:
                        cleaned = str(insight["free_space"]).replace("%", "").strip()
                        insight["free_space"] = int(cleaned)
                    except Exception:
                        pass
                
                insight["timestamp"] = timestamp
                insight["frame_id"] = frame_count
                
                # 3. Heatmap (use the first frame)
                logger.debug(f"Generating heatmap for frame {frame_count}")
                heatmap = crowd_analysis.generate_heatmap(frame)
                processed_frame = crowd_analysis.overlay_heatmap(frame, heatmap)
                
                # 4. Save processed frame to disk for timeline
                PROCESSED_DIR = "/home/ubuntu/irisv3_data/frames"
                video_frames_dir = os.path.join(PROCESSED_DIR, filename)
                os.makedirs(video_frames_dir, exist_ok=True)
                save_path = os.path.join(video_frames_dir, f"{frame_count}.jpg")
                cv2.imwrite(save_path, processed_frame)

                # 5. Encode to Base64 for WebSocket
                _, buffer = cv2.imencode('.jpg', processed_frame)
                jpg_as_text = base64.b64encode(buffer).decode('utf-8')
                
                # 5. Send
                payload = {
                    "type": "frame",
                    "image": f"data:image/jpeg;base64,{jpg_as_text}",
                    "insight": insight
                }
                await websocket.send_json(payload)
                logger.info(f"Sent frame {frame_count} to client")
                
                # 6. Save to DB
                if "error" not in insight:
                    db_insight = CrowdInsight(
                        video_filename=filename,
                        frame_id=frame_count,
                        people_count=insight.get("count", 0),
                        density_label=insight.get("density", "unknown"),
                        movement=insight.get("movement", "unknown"),
                        flow_rate=insight.get("flow_rate", 0),
                        congestion_level=insight.get("congestion_level", 0),
                        free_space=insight.get("free_space", 0),
                        demographics=insight.get("demographics", ""),
                        behavior=insight.get("behavior", ""),
                        alerts=json.dumps(insight.get("alerts", []))
                    )
                    db.add(db_insight)
                    db.commit()
                
                # Cleanup temp files
                for p in frame_paths:
                    if os.path.exists(p):
                        os.remove(p)
                
                # Since we consumed extra frames, we should account for them in frame_count?
                # or just let the loop continue. We consumed +2 frames. 
                # frame_count is just an ID here.
            
            frame_count += 1
            await asyncio.sleep(0.01) # Small yield
            
        cap.release()
        await websocket.send_json({"type": "complete"})
    
    finally:
        db.close()
