# Crowd Monitor

## Setup

### Backend

1. Navigate to `backend` directory.
2. Install dependencies (if not already done):
   ```bash
   pip install -r requirements.txt
   ```
3. Create a `.env` file in `backend/` with your Gemini API key:
   ```
   GENAI_API_KEY=your_key
   ```
4. Place your source videos in `backend/videos/`.
5. Run the server:
   ```bash
   python main.py
   ```

### Frontend

1. Navigate to `frontend` directory.
2. Install dependencies:
   ```bash
   yarn install
   ```
3. Run the development server:
   ```bash
   yarn dev
   ```

## Usage

1. Open http://localhost:5173 (or whatever Vite outputs).
2. Select a video from the left sidebar.
3. Click "Start Analysis".
4. View the processed video frames with heatmap overlay in the center.
5. See real-time insights and alerts on the right.

