package types

type ETCDStorageType int

const (
	ETCD ETCDStorageType = iota
)

const (
	defaultETCDStorageType = ETCD
)

var (
	etcdStorageTypeString = map[string]ETCDStorageType{
		"etcd": ETCD,
	}
)

// ParseETCDStorageType returns the ETCDStorageType given a string representation of the type
func ParseETCDStorageType(s string) ETCDStorageType {
	if storageType, ok := etcdStorageTypeString[s]; ok {
		return storageType
	}

	// TODO: we have to decide what to do in this situation
	return defaultETCDStorageType
}