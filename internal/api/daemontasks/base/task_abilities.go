package base

import "github.com/gameap/gameap/internal/domain"

// DaemonTaskTypeAbilities maps task types to required abilities.
// If a task type is not in this map, access is forbidden for regular users.
var DaemonTaskTypeAbilities = map[domain.DaemonTaskType][]domain.AbilityName{
	domain.DaemonTaskTypeServerStart:   {domain.AbilityNameGameServerCommon, domain.AbilityNameGameServerStart},
	domain.DaemonTaskTypeServerStop:    {domain.AbilityNameGameServerCommon, domain.AbilityNameGameServerStop},
	domain.DaemonTaskTypeServerRestart: {domain.AbilityNameGameServerCommon, domain.AbilityNameGameServerRestart},
	domain.DaemonTaskTypeServerUpdate:  {domain.AbilityNameGameServerCommon, domain.AbilityNameGameServerUpdate},
	domain.DaemonTaskTypeServerInstall: {domain.AbilityNameGameServerCommon, domain.AbilityNameGameServerUpdate},
}
