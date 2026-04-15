package gettoken

type DaemonConnectionChecker interface {
	IsConnectedAnywhere(nodeID uint64) bool
}
