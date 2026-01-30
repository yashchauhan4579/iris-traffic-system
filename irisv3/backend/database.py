from sqlalchemy import create_engine, Column, Integer, String, Float, DateTime, Text
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker
import datetime
import os

# Define external paths
DATA_DIR = "/home/ubuntu/irisv3_data"
DB_PATH = os.path.join(DATA_DIR, "db", "crowd_monitor.db")

os.makedirs(os.path.dirname(DB_PATH), exist_ok=True)

SQLALCHEMY_DATABASE_URL = f"sqlite:///{DB_PATH}"

engine = create_engine(
    SQLALCHEMY_DATABASE_URL, connect_args={"check_same_thread": False}
)
SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)

Base = declarative_base()

class CrowdInsight(Base):
    __tablename__ = "crowd_insights"

    id = Column(Integer, primary_key=True, index=True)
    video_filename = Column(String, index=True)
    timestamp = Column(DateTime, default=datetime.datetime.utcnow)
    frame_id = Column(Integer)
    people_count = Column(Integer)
    density_label = Column(String) # low, medium, high, critical
    movement = Column(String) # static, moving
    free_space = Column(Integer) # 0-100%
    flow_rate = Column(Integer) # people per minute
    congestion_level = Column(Integer) # 0-10
    demographics = Column(String)
    behavior = Column(Text)
    alerts = Column(Text) # JSON string of alerts

class VideoMetadata(Base):
    __tablename__ = "videos"
    
    id = Column(Integer, primary_key=True, index=True)
    filename = Column(String, unique=True, index=True)
    status = Column(String, default="idle")
    processed_frames_path = Column(String)

Base.metadata.create_all(bind=engine)

def get_db():
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()
