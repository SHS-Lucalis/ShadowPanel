package filters

const (
	DefaultLimit  uint64 = 20
	DefaultOffset uint64 = 0
)

var DefaultPagination = &Pagination{
	Limit:  DefaultLimit,
	Offset: DefaultOffset,
}

type Pagination struct {
	Limit  uint64
	Offset uint64
}

func NewPagination(limit, offset uint64) *Pagination {
	return &Pagination{
		Limit:  limit,
		Offset: offset,
	}
}
