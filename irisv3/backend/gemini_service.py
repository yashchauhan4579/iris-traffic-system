import os
import google.generativeai as genai
from PIL import Image
import io
import json
import logging

logger = logging.getLogger(__name__)

# Configure API key
GENAI_API_KEY = os.getenv("GENAI_API_KEY")
if GENAI_API_KEY:
    genai.configure(api_key=GENAI_API_KEY)

async def analyze_frames(image_paths):
    """
    Sends images to Gemini Flash to extract crowd insights.
    """
    if not GENAI_API_KEY:
        logger.warning("GENAI_API_KEY is not set.")
        return {
            "count": 0,
            "density": "unknown",
            "movement": "unknown",
            "behavior": "Simulated: API Key missing",
            "alerts": []
        }

    try:
        model = genai.GenerativeModel('gemini-2.5-flash') # Revert to 2.0-flash-exp or 1.5-flash if 2.5 fails
        
        # Open images
        images = []
        for path in image_paths:
            if os.path.exists(path):
                images.append(Image.open(path))
            else:
                 logger.warning(f"Image not found: {path}")

        if not images:
             return {
                "count": 0,
                "density": "error",
                "movement": "error",
                "behavior": "No images",
                "alerts": ["Image Error"]
            }
        
        logger.debug(f"Sending {len(images)} images to Gemini for analysis")
        
        prompt = """
        Analyze these consecutive frames for crowd monitoring. 
        Provide a JSON response with the following fields:
        
        1. count: estimated number of people (integer)
        2. density: 'low', 'medium', 'high', 'critical'
           - condition is based on crowd concentration: if no visible free space and too much crowd then 'critical', if visible space then 'low', etc.
        3. movement: 'static' or 'moving'
           - based on analysis of the frames, determine if the crowd is generally moving or static.
        4. flow_rate: integer (estimated number of people passing through per minute)
        5. congestion_level: integer 0-10 (0=empty, 10=crush load)
        6. free_space: integer 0-100 (percentage of visible area that is unoccupied)
        7. behavior: brief description of crowd behavior
        8. alerts: list of specific safety issues (e.g., 'congestion', 'fallen person', 'none')
        9. demographics: brief summary of crowd composition (e.g., 'mixed ages', 'mostly students')
        
        Return ONLY valid JSON.
        """
        
        # Pass all images + prompt
        content = [prompt] + images
        response = model.generate_content(content)
        
        logger.debug(f"Gemini response received: {response.text[:100]}...")
        text = response.text.replace('```json', '').replace('```', '')
        return json.loads(text)
        
    except Exception as e:
        logger.error(f"Gemini API Error: {e}", exc_info=True)
        return {
            "count": 0,
            "density": "error",
            "movement": "error",
            "behavior": "Error processing frame",
            "alerts": ["API Error"]
        }
