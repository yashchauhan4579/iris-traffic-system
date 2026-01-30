package decoder

import (
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// HardwareType represents the type of hardware acceleration available
type HardwareType string

const (
	HWNone        HardwareType = "none"         // Software only
	HWNVIDIAJetson HardwareType = "nvidia_jetson" // Jetson (nvv4l2dec)
	HWNVIDIADesktop HardwareType = "nvidia_desktop" // Desktop GPU (NVDEC/CUVID)
	HWIntelVAAPI   HardwareType = "intel_vaapi"   // Intel Quick Sync
	HWAMVAAPI     HardwareType = "amd_vaapi"     // AMD VCN
	HWApple       HardwareType = "apple"         // VideoToolbox
)

// BackendType represents the decoding backend to use
type BackendType string

const (
	BackendFFmpeg    BackendType = "ffmpeg"
	BackendGStreamer BackendType = "gstreamer"
)

// HardwareInfo contains detected hardware capabilities
type HardwareInfo struct {
	Type           HardwareType
	Backend        BackendType
	GPUName        string
	FFmpegPath     string
	GStreamerPath  string
	FFmpegDecoders []string // Available hardware decoders in FFmpeg
	GSTDecoders    []string // Available GStreamer decoder elements
}

// DetectHardware probes the system for available hardware acceleration
func DetectHardware() *HardwareInfo {
	info := &HardwareInfo{
		Type:    HWNone,
		Backend: BackendFFmpeg, // Default
	}

	// Check for FFmpeg
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		info.FFmpegPath = path
		info.FFmpegDecoders = detectFFmpegDecoders()
	}

	// Check for GStreamer
	if path, err := exec.LookPath("gst-launch-1.0"); err == nil {
		info.GStreamerPath = path
		info.GSTDecoders = detectGStreamerDecoders()
	}

	// Detect hardware type
	info.Type, info.GPUName = detectGPU()

	// Select best backend based on hardware
	info.Backend = selectBestBackend(info)

	log.Printf("🔍 Hardware detection:")
	log.Printf("   GPU: %s (%s)", info.GPUName, info.Type)
	log.Printf("   Backend: %s", info.Backend)
	log.Printf("   FFmpeg decoders: %v", info.FFmpegDecoders)
	if len(info.GSTDecoders) > 0 {
		log.Printf("   GStreamer decoders: %v", info.GSTDecoders)
	}

	return info
}

func detectGPU() (HardwareType, string) {
	// Check for NVIDIA Jetson first (has /etc/nv_tegra_release)
	if _, err := os.Stat("/etc/nv_tegra_release"); err == nil {
		// Read Jetson model
		if data, err := os.ReadFile("/proc/device-tree/model"); err == nil {
			return HWNVIDIAJetson, strings.TrimSpace(string(data))
		}
		return HWNVIDIAJetson, "NVIDIA Jetson"
	}

	// Check for NVIDIA desktop GPU via nvidia-smi
	if out, err := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader").Output(); err == nil {
		gpuName := strings.TrimSpace(string(out))
		if gpuName != "" {
			return HWNVIDIADesktop, gpuName
		}
	}

	// Check for Intel GPU (VAAPI)
	if _, err := os.Stat("/dev/dri/renderD128"); err == nil {
		if out, err := exec.Command("lspci", "-nn").Output(); err == nil {
			lspciOut := string(out)
			if strings.Contains(lspciOut, "Intel") && strings.Contains(lspciOut, "VGA") {
				return HWIntelVAAPI, "Intel Integrated Graphics"
			}
			if strings.Contains(lspciOut, "AMD") && strings.Contains(lspciOut, "VGA") {
				return HWAMVAAPI, "AMD GPU"
			}
		}
	}

	// Check for Apple Silicon
	if runtime.GOOS == "darwin" {
		if out, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
			return HWApple, strings.TrimSpace(string(out))
		}
	}

	return HWNone, "CPU (Software)"
}

func detectFFmpegDecoders() []string {
	var decoders []string

	out, err := exec.Command("ffmpeg", "-hide_banner", "-decoders").Output()
	if err != nil {
		return decoders
	}

	output := string(out)

	// Check for hardware decoders
	hwDecoders := []string{
		"h264_cuvid",      // NVIDIA NVDEC
		"hevc_cuvid",      // NVIDIA NVDEC
		"h264_nvmpi",      // Jetson (older)
		"h264_v4l2m2m",    // V4L2 (Jetson, RPi)
		"h264_vaapi",      // Intel/AMD VAAPI
		"hevc_vaapi",      // Intel/AMD VAAPI
		"h264_videotoolbox", // macOS
		"hevc_videotoolbox", // macOS
	}

	for _, dec := range hwDecoders {
		if strings.Contains(output, dec) {
			decoders = append(decoders, dec)
		}
	}

	return decoders
}

func detectGStreamerDecoders() []string {
	var decoders []string

	out, err := exec.Command("gst-inspect-1.0", "--print-plugin-auto-install-info").Output()
	if err != nil {
		// Try alternative method
		out, err = exec.Command("gst-inspect-1.0").Output()
		if err != nil {
			return decoders
		}
	}

	output := string(out)

	// Check for hardware decoder elements
	gstDecoders := []string{
		"nvv4l2decoder",   // Jetson native
		"nvdec",           // NVIDIA desktop
		"nvh264dec",       // NVIDIA H264
		"nvh265dec",       // NVIDIA H265
		"vaapih264dec",    // Intel/AMD VAAPI
		"vaapih265dec",    // Intel/AMD VAAPI
		"vtdec",           // macOS VideoToolbox
		"v4l2h264dec",     // V4L2 hardware
		"omxh264dec",      // OpenMAX (older Jetson)
	}

	for _, dec := range gstDecoders {
		if strings.Contains(output, dec) {
			decoders = append(decoders, dec)
		}
	}

	return decoders
}

func selectBestBackend(info *HardwareInfo) BackendType {
	switch info.Type {
	case HWNVIDIAJetson:
		// Prefer GStreamer on Jetson for nvv4l2decoder
		for _, dec := range info.GSTDecoders {
			if dec == "nvv4l2decoder" || dec == "nvdec" {
				return BackendGStreamer
			}
		}
		// Fall back to FFmpeg with v4l2
		return BackendFFmpeg

	case HWNVIDIADesktop:
		// FFmpeg with NVDEC is excellent on desktop
		for _, dec := range info.FFmpegDecoders {
			if strings.Contains(dec, "cuvid") {
				return BackendFFmpeg
			}
		}
		// GStreamer nvdec is also good
		for _, dec := range info.GSTDecoders {
			if dec == "nvdec" || dec == "nvh264dec" {
				return BackendGStreamer
			}
		}
		return BackendFFmpeg

	case HWIntelVAAPI, HWAMVAAPI:
		// VAAPI works well with both, prefer FFmpeg for simplicity
		return BackendFFmpeg

	case HWApple:
		// FFmpeg VideoToolbox is well supported
		return BackendFFmpeg

	default:
		// Software decode - FFmpeg is reliable
		return BackendFFmpeg
	}
}

// GetFFmpegHWAccelArgs returns FFmpeg arguments for hardware acceleration
func (h *HardwareInfo) GetFFmpegHWAccelArgs() []string {
	switch h.Type {
	case HWNVIDIAJetson:
		// Jetson uses V4L2 or specific decoders
		for _, dec := range h.FFmpegDecoders {
			if dec == "h264_v4l2m2m" {
				return []string{"-c:v", "h264_v4l2m2m"}
			}
		}
		return nil

	case HWNVIDIADesktop:
		// Desktop NVIDIA uses CUVID/NVDEC
		for _, dec := range h.FFmpegDecoders {
			if dec == "h264_cuvid" {
				return []string{"-hwaccel", "cuda", "-c:v", "h264_cuvid"}
			}
		}
		return nil

	case HWIntelVAAPI, HWAMVAAPI:
		for _, dec := range h.FFmpegDecoders {
			if dec == "h264_vaapi" {
				return []string{"-hwaccel", "vaapi", "-hwaccel_device", "/dev/dri/renderD128"}
			}
		}
		return nil

	case HWApple:
		for _, dec := range h.FFmpegDecoders {
			if dec == "h264_videotoolbox" {
				return []string{"-hwaccel", "videotoolbox"}
			}
		}
		return nil

	default:
		return nil
	}
}

// GetGStreamerDecoderElement returns the best GStreamer decoder element
func (h *HardwareInfo) GetGStreamerDecoderElement() string {
	switch h.Type {
	case HWNVIDIAJetson:
		for _, dec := range h.GSTDecoders {
			if dec == "nvv4l2decoder" {
				return "nvv4l2decoder"
			}
		}
		return "avdec_h264" // Software fallback

	case HWNVIDIADesktop:
		for _, dec := range h.GSTDecoders {
			if dec == "nvdec" || dec == "nvh264dec" {
				return dec
			}
		}
		return "avdec_h264"

	case HWIntelVAAPI, HWAMVAAPI:
		for _, dec := range h.GSTDecoders {
			if dec == "vaapih264dec" {
				return "vaapih264dec"
			}
		}
		return "avdec_h264"

	default:
		return "avdec_h264"
	}
}

