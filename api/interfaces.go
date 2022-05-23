package api

type KubeadmConfigResourceVersionDependant interface {
	GetKubeadmConfigResourceVersion() string
	SetKubeadmConfigResourceVersion(string)
}
