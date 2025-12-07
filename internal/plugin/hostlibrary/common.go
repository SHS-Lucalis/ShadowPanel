package hostlibrary

import (
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/pkg/plugin/sdk/common"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/samber/lo"
)

func uintsFromUint64s(ids []uint64) []uint {
	return lo.Map(ids, func(id uint64, _ int) uint {
		return uint(id)
	})
}

func uintPtrsFromUint64s(ids []uint64) []*uint {
	return lo.Map(ids, func(id uint64, _ int) *uint {
		v := uint(id)

		return &v
	})
}

func convertSorting(sorting []*common.Sorting) []filters.Sorting {
	if sorting == nil {
		return nil
	}

	return lo.Map(sorting, func(s *common.Sorting, _ int) filters.Sorting {
		direction := filters.SortDirectionAsc
		if s.Descending {
			direction = filters.SortDirectionDesc
		}

		return filters.Sorting{
			Field:     s.Field,
			Direction: direction,
		}
	})
}

func uintPtrFromUint64Ptr(v *uint64) *uint {
	if v == nil {
		return nil
	}

	return lo.ToPtr(uint(*v))
}

func uint64PtrFromUintPtr(v *uint) *uint64 {
	if v == nil {
		return nil
	}

	return lo.ToPtr(uint64(*v))
}

var protoToEntityType = map[proto.EntityType]domain.EntityType{
	proto.EntityType_ENTITY_TYPE_UNSPECIFIED:        domain.EntityTypeEmpty,
	proto.EntityType_ENTITY_TYPE_USER:               domain.EntityTypeUser,
	proto.EntityType_ENTITY_TYPE_NODE:               domain.EntityTypeNode,
	proto.EntityType_ENTITY_TYPE_CLIENT_CERTIFICATE: domain.EntityTypeClientCertificate,
	proto.EntityType_ENTITY_TYPE_GAME:               domain.EntityTypeGame,
	proto.EntityType_ENTITY_TYPE_GAME_MOD:           domain.EntityTypeGameMod,
	proto.EntityType_ENTITY_TYPE_SERVER:             domain.EntityTypeServer,
	proto.EntityType_ENTITY_TYPE_ROLE:               domain.EntityTypeRole,
}

var entityTypeToProto = map[domain.EntityType]proto.EntityType{
	domain.EntityTypeEmpty:             proto.EntityType_ENTITY_TYPE_UNSPECIFIED,
	domain.EntityTypeUser:              proto.EntityType_ENTITY_TYPE_USER,
	domain.EntityTypeNode:              proto.EntityType_ENTITY_TYPE_NODE,
	domain.EntityTypeClientCertificate: proto.EntityType_ENTITY_TYPE_CLIENT_CERTIFICATE,
	domain.EntityTypeGame:              proto.EntityType_ENTITY_TYPE_GAME,
	domain.EntityTypeGameMod:           proto.EntityType_ENTITY_TYPE_GAME_MOD,
	domain.EntityTypeServer:            proto.EntityType_ENTITY_TYPE_SERVER,
	domain.EntityTypeRole:              proto.EntityType_ENTITY_TYPE_ROLE,
}

func entityTypeFromProto(et *proto.EntityType) *string {
	if et == nil {
		return nil
	}

	domainET, ok := protoToEntityType[*et]
	if !ok || domainET == domain.EntityTypeEmpty {
		return nil
	}

	return lo.ToPtr(string(domainET))
}

func entityTypeToProtoPtr(et *string) *proto.EntityType {
	if et == nil {
		return nil
	}

	protoET, ok := entityTypeToProto[domain.EntityType(*et)]
	if !ok {
		return nil
	}

	return lo.ToPtr(protoET)
}
