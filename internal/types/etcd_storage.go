package types

import (
	"fmt"

	"github.com/spf13/viper"
)

type ETCDStorageType int

const (
	ETCD ETCDStorageType = iota
	KineMySQL
)

var etcdStorageTypeString = map[string]ETCDStorageType{"etcd": ETCD, "kine-mysql": KineMySQL}

func (s ETCDStorageType) String() string {
	return [...]string{"etcd", "kine-mysql"}[s]
}

// ParseETCDStorageType returns the ETCDStorageType given a string representation of the type.
func ParseETCDStorageType(s string) ETCDStorageType {
	if storageType, ok := etcdStorageTypeString[s]; ok {
		return storageType
	}

	panic(fmt.Errorf("unsupported storage type %s", s))
}

// ParseETCDEndpoint returns the default ETCD endpoints used to interact with the Tenant Control Plane backing storage.
func ParseETCDEndpoint(conf *viper.Viper) string {
	switch ParseETCDStorageType(conf.GetString("etcd-storage-type")) {
	case ETCD:
		return conf.GetString("etcd-endpoints")
	case KineMySQL:
		return "127.0.0.1:2379"
	default:
		panic("unsupported storage type")
	}
}
