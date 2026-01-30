// Package api provides HTTP API handlers for MagicNetwork
package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/irisdrone/magicnetwork/internal/wireguard"
)

// API handles HTTP requests
type API struct {
	wg     *wireguard.Server
	apiKey string
}

// NewAPI creates a new API handler
func NewAPI(wg *wireguard.Server, apiKey string) *API {
	return &API{
		wg:     wg,
		apiKey: apiKey,
	}
}

// AuthMiddleware validates API key
func (a *API) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// Also check X-API-Key header
			authHeader = c.GetHeader("X-API-Key")
		}

		// Remove "Bearer " prefix if present
		authHeader = strings.TrimPrefix(authHeader, "Bearer ")

		if authHeader != a.apiKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or missing API key",
			})
			return
		}

		c.Next()
	}
}

// RegisterPeerRequest for peer registration
type RegisterPeerRequest struct {
	ID        string `json:"id" binding:"required"`
	Name      string `json:"name" binding:"required"`
	PublicKey string `json:"public_key" binding:"required"`
}

// RegisterPeer handles peer registration
// POST /api/peers
func (a *API) RegisterPeer(c *gin.Context) {
	var req RegisterPeerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	peer, err := a.wg.RegisterPeer(req.ID, req.Name, req.PublicKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	cfg := a.wg.GetConfig()

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"peer": gin.H{
			"id":          peer.ID,
			"name":        peer.Name,
			"assigned_ip": peer.AssignedIP + "/24",
			"allowed_ips": peer.AllowedIPs,
			"created_at":  peer.CreatedAt,
		},
		"server": gin.H{
			"public_key":  cfg.PublicKey,
			"endpoint":    c.Request.Host, // Will be replaced by actual endpoint
			"listen_port": cfg.ListenPort,
			"server_ip":   strings.Split(cfg.Address, "/")[0],
		},
	})
}

// GetPeers returns all registered peers
// GET /api/peers
func (a *API) GetPeers(c *gin.Context) {
	// Update status first
	a.wg.UpdatePeerStatus()

	peers := a.wg.GetPeers()
	c.JSON(http.StatusOK, gin.H{
		"peers": peers,
		"count": len(peers),
	})
}

// GetPeer returns a specific peer
// GET /api/peers/:pubkey
func (a *API) GetPeer(c *gin.Context) {
	pubKey := c.Param("pubkey")
	
	a.wg.UpdatePeerStatus()
	
	peer := a.wg.GetPeer(pubKey)
	if peer == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Peer not found"})
		return
	}

	c.JSON(http.StatusOK, peer)
}

// RemovePeer removes a peer
// DELETE /api/peers/:pubkey
func (a *API) RemovePeer(c *gin.Context) {
	pubKey := c.Param("pubkey")

	if err := a.wg.RemovePeer(pubKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetStatus returns server status
// GET /api/status
func (a *API) GetStatus(c *gin.Context) {
	cfg := a.wg.GetConfig()
	
	a.wg.UpdatePeerStatus()
	peers := a.wg.GetPeers()

	// Count connected peers
	connected := 0
	for _, p := range peers {
		if !p.LastSeen.IsZero() {
			connected++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "running",
		"interface":  wireguard.InterfaceName,
		"public_key": cfg.PublicKey,
		"address":    cfg.Address,
		"port":       cfg.ListenPort,
		"peers": gin.H{
			"total":     len(peers),
			"connected": connected,
		},
	})
}

// GetServerInfo returns server connection info (public endpoint)
// GET /api/info
func (a *API) GetServerInfo(c *gin.Context) {
	cfg := a.wg.GetConfig()

	c.JSON(http.StatusOK, gin.H{
		"public_key":  cfg.PublicKey,
		"listen_port": cfg.ListenPort,
		"server_ip":   strings.Split(cfg.Address, "/")[0],
	})
}

