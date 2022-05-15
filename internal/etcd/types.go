// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package etcd

type Config struct {
	ETCDCertificate []byte
	ETCDPrivateKey  []byte
	ETCDCA          []byte
	Endpoints       []string
}

type Permission struct {
	Type     int    `json:"type,omitempty"`
	Key      string `json:"key,omitempty"`
	RangeEnd string `json:"rangeEnd,omitempty"`
}

func (in *Permission) DeepCopyInto(out *Permission) {
	*out = *in
}

func (in *Permission) DeepCopy() *Permission {
	if in == nil {
		return nil
	}
	out := new(Permission)
	in.DeepCopyInto(out)

	return out
}

type Role struct {
	Name        string       `json:"name"`
	Permissions []Permission `json:"permissions,omitempty"`
	Exists      bool         `json:"exists"`
}

func (in *Role) DeepCopyInto(out *Role) {
	*out = *in
}

func (in *Role) DeepCopy() *Role {
	if in == nil {
		return nil
	}
	out := new(Role)
	in.DeepCopyInto(out)

	return out
}

type User struct {
	Name   string   `json:"name"`
	Roles  []string `json:"roles,omitempty"`
	Exists bool     `json:"exists"`
}

func (in *User) DeepCopyInto(out *User) {
	*out = *in
}

func (in *User) DeepCopy() *User {
	if in == nil {
		return nil
	}
	out := new(User)
	in.DeepCopyInto(out)

	return out
}
