package domain

type EntityType string

const (
	EntityTypeEmpty             EntityType = ""
	EntityTypeUser              EntityType = "Gameap\\Models\\User"
	EntityTypeNode              EntityType = "Gameap\\Models\\DedicatedServer"
	EntityTypeClientCertificate EntityType = "Gameap\\Models\\ClientCertificate"
	EntityTypeGame              EntityType = "Gameap\\Models\\Game"
	EntityTypeGameMod           EntityType = "Gameap\\Models\\GameMod"
	EntityTypeServer            EntityType = "Gameap\\Models\\Server"
	EntityTypeRole              EntityType = "roles"
)
