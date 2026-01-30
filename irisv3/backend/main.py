import os
import cv2
import json
import asyncio
import base64
from fastapi import FastAPI, UploadFile, File, HTTPException, BackgroundTasks, WebSocket, WebSocketDisconnect
from fastapi.responses import JSONResponse, FileResponse
from fastapi.middleware.cors import CORSMiddleware
from typing import List
import uvicorn
import logging
from dotenv import load_dotenv

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Load env vars
load_dotenv()

app = FastAPI()

@app.on_event("startup")
async def startup_event():
    logger.info("Starting Crowd Monitor Backend")

# CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

VIDEO_DIR = "videos"
PROCESSED_DIR = "/home/ubuntu/irisv3_data/frames"

os.makedirs(VIDEO_DIR, exist_ok=True)
os.makedirs(PROCESSED_DIR, exist_ok=True)

# Database
from database import get_db, CrowdInsight, VideoMetadata, SessionLocal
from sqlalchemy.orm import Session
from fastapi import Depends

@app.get("/videos")
async def list_videos():
    files = [f for f in os.listdir(VIDEO_DIR) if f.endswith(('.mp4', '.avi', '.mov'))]
    return {"videos": files}

@app.post("/upload")
async def upload_video(file: UploadFile = File(...)):
    file_path = os.path.join(VIDEO_DIR, file.filename)
    with open(file_path, "wb") as buffer:
        content = await file.read()
        buffer.write(content)
    return {"filename": file.filename}

@app.get("/video/{filename}")
async def get_video(filename: str):
    file_path = os.path.join(VIDEO_DIR, filename)
    if not os.path.exists(file_path):
        raise HTTPException(status_code=404, detail="Video not found")
    return FileResponse(file_path)

@app.get("/processed_frame/{filename}/{frame_id}")
async def get_processed_frame(filename: str, frame_id: str):
    # Ensure filename is safe (basic check)
    filename = os.path.basename(filename) 
    frame_path = os.path.join(PROCESSED_DIR, filename, f"{frame_id}.jpg")
    
    if not os.path.exists(frame_path):
        raise HTTPException(status_code=404, detail="Frame not found")
    return FileResponse(frame_path)

@app.websocket("/ws/analyze/{filename}")
async def websocket_analyze(websocket: WebSocket, filename: str):
    await websocket.accept()
    video_path = os.path.join(VIDEO_DIR, filename)
    
    logger.info(f"WebSocket connected for video: {filename}")
    
    if not os.path.exists(video_path):
        logger.warning(f"Video not found: {filename}")
        await websocket.close(code=4004, reason="Video not found")
        return

    import video_processing
    try:
        await video_processing.stream_video_analysis(video_path, websocket)
    except WebSocketDisconnect:
        logger.info(f"Client disconnected for {filename}")
    except Exception as e:
        logger.error(f"Error in websocket stream for {filename}: {e}", exc_info=True)
        try:
            await websocket.close(code=1011, reason=str(e))
        except:
            pass

@app.get("/insights/{filename}")
async def get_insights(filename: str, db: Session = Depends(get_db)):
    # Fetch from DB
    insights = db.query(CrowdInsight).filter(CrowdInsight.video_filename == filename).order_by(CrowdInsight.timestamp).all()
    
    if not insights:
        return {"status": "not_found", "insights": []}
    
    formatted_insights = []
    for i in insights:
        formatted_insights.append({
            "count": i.people_count,
            "density": i.density_label,
            "movement": i.movement,
            "flow_rate": i.flow_rate,
            "free_space": i.free_space,
            "congestion": i.congestion_level,
            "demographics": i.demographics,
            "behavior": i.behavior,
            "alerts": json.loads(i.alerts) if i.alerts else [],
            "timestamp": i.timestamp.isoformat(),
            "frame_id": i.frame_id
        })
        
    return {"status": "success", "insights": formatted_insights}

if __name__ == "__main__":
    uvicorn.run("main:app", host="0.0.0.0", port=8000, reload=True)
