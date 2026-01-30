import cv2
import numpy as np
import logging
import os
import sys

logger = logging.getLogger(__name__)

# Try to import model dependencies
try:
    import torch
    from torch.autograd import Variable
    import torchvision.transforms as standard_transforms
    from PIL import Image
    import matplotlib
    matplotlib.use('Agg')
    from matplotlib import pyplot as plt
    MODEL_AVAILABLE = True
except ImportError:
    MODEL_AVAILABLE = False
    logger.warning("PyTorch/Matplotlib not available. Using fallback heatmap generation.")

# Try to import model config if available
_model_net = None
_model_path = None
_net = None
_device = None
_img_transform = None

def _try_load_model():
    """Try to load the crowd counting model from crowdanalysis directory"""
    global _model_net, _model_path, _net, _device, _img_transform
    
    if not MODEL_AVAILABLE:
        return False
    
    try:
        # Add crowdanalysis to path
        crowdanalysis_path = os.path.join(os.path.dirname(__file__), 'crowdanalysis')
        if os.path.exists(crowdanalysis_path):
            sys.path.insert(0, crowdanalysis_path)
        
        from test_config import cfg
        
        _model_net = cfg.NET
        _model_path = cfg.MODEL_PATH
        
        # Resolve relative path - if it's relative, make it relative to crowdanalysis directory
        if not os.path.isabs(_model_path):
            _model_path = os.path.join(crowdanalysis_path, _model_path)
        
        # Check if model file exists
        if not os.path.exists(_model_path):
            logger.warning(f"Model file not found: {_model_path}. Using fallback.")
            return False
        
        # Setup device
        if torch.cuda.is_available():
            _device = torch.device('cuda')
        else:
            _device = torch.device('cpu')
        
        # Load dataset config for transforms
        data_mode = cfg.DATASET
        if data_mode == 'SHHA':
            from datasets.SHHA.setting import cfg_data
        elif data_mode == 'SHHB':
            from datasets.SHHB.setting import cfg_data
        elif data_mode == 'QNRF':
            from datasets.QNRF.setting import cfg_data
        elif data_mode == 'UCF50':
            from datasets.UCF50.setting import cfg_data
        
        mean_std = cfg_data.MEAN_STD
        _img_transform = standard_transforms.Compose([
            standard_transforms.ToTensor(),
            standard_transforms.Normalize(*mean_std)
        ])
        
        # Load model
        if 'LCM' in _model_net:
            from models.CC_LCM import CrowdCounter
        elif 'DM' in _model_net:
            from models.CC_DM import CrowdCounter
        else:
            logger.warning(f"Unknown model type: {_model_net}")
            return False
        
        _net = CrowdCounter(cfg.GPU_ID, _model_net, pretrained=False)
        
        # Load weights
        if len(cfg.GPU_ID) == 1:
            _net.load_state_dict(torch.load(_model_path, map_location=_device))
        else:
            # Multi-GPU model conversion
            from collections import OrderedDict
            state_dict = torch.load(_model_path, map_location=_device)
            new_state_dict = OrderedDict()
            for k, v in state_dict.items():
                name = k[0:3] + k[10:]  # remove 'module.'
                new_state_dict[name] = v
            _net.load_state_dict(new_state_dict)
        
        _net.to(_device)
        _net.eval()
        
        logger.info(f"Model loaded successfully: {_model_net}")
        return True
        
    except Exception as e:
        logger.warning(f"Could not load model: {e}. Using fallback heatmap generation.")
        return False

# Try to load model on import
_model_loaded = _try_load_model()

def generate_heatmap(image):
    """
    Generates crowd density heatmap using the model if available, otherwise falls back to HOG.
    Uses matplotlib jet colormap like demo.py
    """
    try:
        # Try to use model if available
        if _model_loaded and _net is not None:
            return _generate_heatmap_with_model(image)
        else:
            return _generate_heatmap_fallback(image)
    except Exception as e:
        logger.error(f"Error generating heatmap: {e}", exc_info=True)
        return _generate_heatmap_fallback(image)

def _generate_heatmap_with_model(image):
    """Generate heatmap using the deep learning model (like demo.py)"""
    # Convert for model
    img_pil = Image.fromarray(cv2.cvtColor(image, cv2.COLOR_BGR2RGB))
    if img_pil.mode != 'RGB':
        img_pil = img_pil.convert('RGB')
    
    img_tensor = _img_transform(img_pil)
    
    with torch.no_grad():
        img_tensor = Variable(img_tensor[None, :, :, :]).to(_device)
        pred_map = _net.test_forward(img_tensor)
    
    # Extract density map
    density_map = pred_map.cpu().data.numpy()[0, 0, :, :]
    
    # Resize if needed (for DM models)
    if 'DM' in _model_net:
        from test_config import cfg
        density_map = cv2.resize(density_map, (density_map.shape[1]*8, density_map.shape[0]*8))
    
    # Generate heatmap using matplotlib (like demo.py lines 594-599)
    norm = plt.Normalize(vmin=np.min(density_map), vmax=np.max(density_map))
    cmap = plt.get_cmap('jet')
    heatmap_rgba = cmap(norm(density_map))
    heatmap_rgb = np.delete(heatmap_rgba, 3, 2)
    heatmap_bgr = cv2.cvtColor((heatmap_rgb * 255).astype(np.uint8), cv2.COLOR_RGB2BGR)
    
    # Resize to match original image
    heatmap_final = cv2.resize(heatmap_bgr, (image.shape[1], image.shape[0]))
    
    logger.debug("Generated heatmap using model")
    return heatmap_final

def _generate_heatmap_fallback(image):
    """Fallback heatmap generation using HOG detector"""
    hog = cv2.HOGDescriptor()
    hog.setSVMDetector(cv2.HOGDescriptor_getDefaultPeopleDetector())
    
    # Resize for speed
    small_img = cv2.resize(image, (640, 480))
    boxes, weights = hog.detectMultiScale(small_img, winStride=(8,8))
    
    logger.debug(f"Detected {len(boxes)} people for heatmap generation (fallback)")
    
    # Create empty density map
    density_map = np.zeros((480, 640), dtype=np.float32)
    
    for (x, y, w, h) in boxes:
        center_x = x + w // 2
        center_y = y + h // 2
        radius = h // 2
        cv2.circle(density_map, (center_x, center_y), radius, (1,), -1)
    
    # Blur the map
    density_map = cv2.GaussianBlur(density_map, (31, 31), 0)
    
    # Normalize
    if density_map.max() > 0:
        density_map = density_map / density_map.max()
    
    # Use matplotlib jet colormap like demo.py if available
    if MODEL_AVAILABLE:
        norm = plt.Normalize(vmin=np.min(density_map), vmax=np.max(density_map))
        cmap = plt.get_cmap('jet')
        heatmap_rgba = cmap(norm(density_map))
        heatmap_rgb = np.delete(heatmap_rgba, 3, 2)
        heatmap_bgr = cv2.cvtColor((heatmap_rgb * 255).astype(np.uint8), cv2.COLOR_RGB2BGR)
        heatmap_final = cv2.resize(heatmap_bgr, (image.shape[1], image.shape[0]))
    else:
        # Fallback to OpenCV colormap
        heatmap_uint8 = (density_map * 255).astype(np.uint8)
        heatmap_color = cv2.applyColorMap(heatmap_uint8, cv2.COLORMAP_JET)
        heatmap_final = cv2.resize(heatmap_color, (image.shape[1], image.shape[0]))
    
    return heatmap_final

def overlay_heatmap(original_image, heatmap, alpha=0.5):
    """
    Overlays heatmap on original image.
    """
    try:
        return cv2.addWeighted(original_image, 1 - alpha, heatmap, alpha, 0)
    except Exception as e:
        logger.error(f"Error overlaying heatmap: {e}", exc_info=True)
        return original_image
