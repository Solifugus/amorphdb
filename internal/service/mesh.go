// Package service provides mesh integration for the AmorphDB service
package service

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/solifugus/amorphdb/internal/config"
	"github.com/solifugus/amorphdb/internal/mesh"
)

// MeshService provides mesh operations via the service interface
type MeshService struct {
	meshManager *mesh.MeshManager
}

// CreateMeshRequest represents a request to create a new mesh
type CreateMeshRequest struct {
	Name string `json:"name"`
}

// JoinMeshRequest represents a request to join an existing mesh
type JoinMeshRequest struct {
	SeedAddress string `json:"seed_address"`
}

// CreateBridgeRequest represents a request to create a bridge to another mesh
type CreateBridgeRequest struct {
	TargetAddress string `json:"target_address"`
}

// DetachRequest represents a request to detach from a mesh or bridge
type DetachRequest struct {
	MeshName string `json:"mesh_name"`
}

// CreateMeshResponse represents the response from creating a mesh
type CreateMeshResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	MeshName    string `json:"mesh_name,omitempty"`
	NodeID      string `json:"node_id,omitempty"`
	IsFounder   bool   `json:"is_founder,omitempty"`
}

// JoinMeshResponse represents the response from joining a mesh
type JoinMeshResponse struct {
	Success       bool               `json:"success"`
	Message       string             `json:"message"`
	NodeIdentity  string             `json:"node_identity,omitempty"`
	MeshInfo      *JoinedMeshInfo    `json:"mesh_info,omitempty"`
	DataPreserved bool               `json:"data_preserved"`
}

// JoinedMeshInfo represents information about the joined mesh
type JoinedMeshInfo struct {
	Name          string `json:"name"`
	AssignedZone  string `json:"assigned_zone,omitempty"`
	MemberCount   int    `json:"member_count"`
	JoinedAt      int64  `json:"joined_at"`
}

// CreateBridgeResponse represents the response from creating a bridge
type CreateBridgeResponse struct {
	Success    bool               `json:"success"`
	Message    string             `json:"message"`
	BridgeInfo *CreatedBridgeInfo `json:"bridge_info,omitempty"`
}

// CreatedBridgeInfo represents information about the created bridge
type CreatedBridgeInfo struct {
	TargetMeshName  string `json:"target_mesh_name"`
	BridgeIdentity  string `json:"bridge_identity"`
	Status          string `json:"status"`
	ConnectedAt     int64  `json:"connected_at"`
}

// DetachResponse represents the response from a detach operation
type DetachResponse struct {
	Success    bool               `json:"success"`
	Message    string             `json:"message"`
	DetachInfo *DetachResponseInfo `json:"detach_info,omitempty"`
}

// DetachResponseInfo represents information about the detach operation
type DetachResponseInfo struct {
	DetachType      string `json:"detach_type"`       // "primary_mesh" or "bridge"
	MeshName        string `json:"mesh_name"`
	PreviousRole    string `json:"previous_role,omitempty"`      // "founder" or "member"
	BridgeIdentity  string `json:"bridge_identity,omitempty"`   // For bridge disconnections
	ZonesMigrated   int    `json:"zones_migrated"`
	DataPreserved   bool   `json:"data_preserved"`
	DisconnectedAt  *int64 `json:"disconnected_at,omitempty"`
}

// MeshStatusRequest represents a request for mesh status
type MeshStatusRequest struct {
	// Empty for now - might add filtering options later
}

// MeshStatusResponse represents the mesh status response
type MeshStatusResponse struct {
	MeshName      string                        `json:"mesh_name"`
	Status        string                        `json:"status"`
	IsFounder     bool                          `json:"is_founder"`
	NodeIdentity  string                        `json:"node_identity"`
	MemberCount   int                           `json:"member_count"`
	ZoneCount     int                           `json:"zone_count"`
	FoundedAt     *int64                        `json:"founded_at,omitempty"`
	Bridges       map[string]BridgeStatusInfo   `json:"bridges"`
}

// BridgeStatusInfo represents bridge connection status information
type BridgeStatusInfo struct {
	Address     string `json:"address"`
	Status      string `json:"status"`
	Identity    string `json:"identity"`
	ConnectedAt *int64 `json:"connected_at,omitempty"`
}

// NewMeshService creates a new mesh service
func NewMeshService(meshManager *mesh.MeshManager) *MeshService {
	return &MeshService{
		meshManager: meshManager,
	}
}

// CreateMesh handles mesh creation requests
func (ms *MeshService) CreateMesh(requestData []byte) ([]byte, error) {
	var req CreateMeshRequest
	if err := json.Unmarshal(requestData, &req); err != nil {
		return ms.errorResponse("Invalid request format: %v", err)
	}

	if req.Name == "" {
		return ms.errorResponse("Mesh name cannot be empty")
	}

	// Validate mesh name
	if err := config.ValidateMeshName(req.Name); err != nil {
		return ms.errorResponse("Invalid mesh name: %v", err)
	}

	// Create the mesh
	err := ms.meshManager.CreateMesh(req.Name)
	if err != nil {
		return ms.errorResponse("Failed to create mesh: %v", err)
	}

	// Get node identity
	nodeIdentity := ms.meshManager.GetNodeIdentity()

	response := CreateMeshResponse{
		Success:   true,
		Message:   fmt.Sprintf("Successfully created mesh '%s'", req.Name),
		MeshName:  req.Name,
		IsFounder: ms.meshManager.IsFounder(),
	}

	if nodeIdentity != nil {
		response.NodeID = nodeIdentity.ID
	}

	return json.Marshal(response)
}

// JoinMesh handles mesh join requests
func (ms *MeshService) JoinMesh(requestData []byte) ([]byte, error) {
	var req JoinMeshRequest
	if err := json.Unmarshal(requestData, &req); err != nil {
		return ms.errorResponseJoin("Invalid request format: %v", err)
	}

	if req.SeedAddress == "" {
		return ms.errorResponseJoin("Seed address cannot be empty")
	}

	// Attempt to join the mesh
	err := ms.meshManager.JoinMesh(req.SeedAddress)
	if err != nil {
		return ms.errorResponseJoin("Failed to join mesh: %v", err)
	}

	// Get updated mesh state
	meshState := ms.meshManager.GetMeshState()
	nodeIdentity := ms.meshManager.GetNodeIdentity()

	// Prepare successful response
	response := JoinMeshResponse{
		Success:       true,
		Message:       fmt.Sprintf("Successfully joined mesh '%s'", meshState.Name),
		DataPreserved: true, // For now, assume data is always preserved
	}

	if nodeIdentity != nil {
		response.NodeIdentity = nodeIdentity.ID
	}

	if meshState.Name != "" {
		response.MeshInfo = &JoinedMeshInfo{
			Name:        meshState.Name,
			MemberCount: meshState.MemberCount,
			JoinedAt:    time.Now().Unix(),
		}

		// Try to get assigned zone info from storage (simplified for now)
		// In a full implementation, this would query the actual zone assignment
		response.MeshInfo.AssignedZone = "auto-assigned"
	}

	return json.Marshal(response)
}

// CreateBridge handles bridge creation requests
func (ms *MeshService) CreateBridge(requestData []byte) ([]byte, error) {
	var req CreateBridgeRequest
	if err := json.Unmarshal(requestData, &req); err != nil {
		return ms.errorResponseBridge("Invalid request format: %v", err)
	}

	if req.TargetAddress == "" {
		return ms.errorResponseBridge("Target address cannot be empty")
	}

	// Attempt to create the bridge
	bridge, err := ms.meshManager.CreateBridge(req.TargetAddress)
	if err != nil {
		return ms.errorResponseBridge("Failed to create bridge: %v", err)
	}

	// Prepare successful response
	response := CreateBridgeResponse{
		Success: true,
		Message: fmt.Sprintf("Successfully created bridge to mesh '%s'", bridge.MeshName),
	}

	if bridge != nil {
		response.BridgeInfo = &CreatedBridgeInfo{
			TargetMeshName: bridge.MeshName,
			BridgeIdentity: bridge.Identity.ID,
			Status:         string(bridge.Status),
			ConnectedAt:    bridge.ConnectedAt.Unix(),
		}
	}

	return json.Marshal(response)
}

// GetMeshStatus returns the current mesh status
func (ms *MeshService) GetMeshStatus(requestData []byte) ([]byte, error) {
	// Parse request (currently empty, but structured for future expansion)
	var req MeshStatusRequest
	if len(requestData) > 0 {
		if err := json.Unmarshal(requestData, &req); err != nil {
			return ms.errorResponse("Invalid request format: %v", err)
		}
	}

	// Get mesh state
	meshState := ms.meshManager.GetMeshState()
	nodeIdentity := ms.meshManager.GetNodeIdentity()
	bridges := ms.meshManager.GetBridgeConnections()

	response := MeshStatusResponse{
		MeshName:    meshState.Name,
		Status:      string(meshState.Status),
		IsFounder:   ms.meshManager.IsFounder(),
		MemberCount: meshState.MemberCount,
		ZoneCount:   meshState.ZoneCount,
		Bridges:     make(map[string]BridgeStatusInfo),
	}

	if nodeIdentity != nil {
		response.NodeIdentity = nodeIdentity.ID
	}

	if !meshState.FoundedAt.IsZero() {
		foundedAt := meshState.FoundedAt.Unix()
		response.FoundedAt = &foundedAt
	}

	// Add bridge information
	for meshName, bridge := range bridges {
		bridgeInfo := BridgeStatusInfo{
			Address:  bridge.Address,
			Status:   string(bridge.Status),
			Identity: bridge.Identity.ID,
		}

		if !bridge.ConnectedAt.IsZero() {
			connectedAt := bridge.ConnectedAt.Unix()
			bridgeInfo.ConnectedAt = &connectedAt
		}

		response.Bridges[meshName] = bridgeInfo
	}

	return json.Marshal(response)
}

// DetachFromMesh handles detach requests from meshes or bridges
func (ms *MeshService) DetachFromMesh(requestData []byte) ([]byte, error) {
	var req DetachRequest
	if err := json.Unmarshal(requestData, &req); err != nil {
		return ms.errorResponseDetach("Invalid request format: %v", err)
	}

	// Get the leave manager (we'll need to add this to MeshManager)
	leaveManager := ms.meshManager.GetLeaveManager()
	if leaveManager == nil {
		return ms.errorResponseDetach("Leave manager not available")
	}

	// Perform the detach operation
	result, err := leaveManager.DetachFromMesh(req.MeshName)
	if err != nil {
		return ms.errorResponseDetach("Failed to detach: %v", err)
	}

	// Prepare successful response
	var message string
	if result.DetachType == "primary_mesh" {
		message = fmt.Sprintf("Successfully detached from mesh '%s' and returned to standalone mode", result.MeshName)
	} else {
		message = fmt.Sprintf("Successfully disconnected bridge to mesh '%s'", result.MeshName)
	}

	response := DetachResponse{
		Success: true,
		Message: message,
	}

	if result != nil {
		response.DetachInfo = &DetachResponseInfo{
			DetachType:      result.DetachType,
			MeshName:        result.MeshName,
			PreviousRole:    result.PreviousRole,
			BridgeIdentity:  result.BridgeIdentity,
			ZonesMigrated:   result.ZonesMigrated,
			DataPreserved:   result.DataPreserved,
			DisconnectedAt:  &result.DisconnectedAt,
		}
	}

	return json.Marshal(response)
}

// IsStandalone returns true if the node is in standalone mode
func (ms *MeshService) IsStandalone() bool {
	meshState := ms.meshManager.GetMeshState()
	return meshState.Status == "standalone"
}

// GetMeshName returns the current mesh name
func (ms *MeshService) GetMeshName() string {
	meshState := ms.meshManager.GetMeshState()
	return meshState.Name
}

// GetNodeIdentity returns the current node identity ID
func (ms *MeshService) GetNodeIdentity() string {
	nodeIdentity := ms.meshManager.GetNodeIdentity()
	if nodeIdentity == nil {
		return ""
	}
	return nodeIdentity.ID
}

// errorResponse creates a standardized error response
func (ms *MeshService) errorResponse(format string, args ...interface{}) ([]byte, error) {
	response := CreateMeshResponse{
		Success: false,
		Message: fmt.Sprintf(format, args...),
	}
	return json.Marshal(response)
}

// errorResponseJoin creates a standardized error response for join operations
func (ms *MeshService) errorResponseJoin(format string, args ...interface{}) ([]byte, error) {
	response := JoinMeshResponse{
		Success: false,
		Message: fmt.Sprintf(format, args...),
	}
	return json.Marshal(response)
}

// errorResponseBridge creates a standardized error response for bridge operations
func (ms *MeshService) errorResponseBridge(format string, args ...interface{}) ([]byte, error) {
	response := CreateBridgeResponse{
		Success: false,
		Message: fmt.Sprintf(format, args...),
	}
	return json.Marshal(response)
}

// errorResponseDetach creates a standardized error response for detach operations
func (ms *MeshService) errorResponseDetach(format string, args ...interface{}) ([]byte, error) {
	response := DetachResponse{
		Success: false,
		Message: fmt.Sprintf(format, args...),
	}
	return json.Marshal(response)
}