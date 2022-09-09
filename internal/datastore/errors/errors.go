// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package errors

import "github.com/pkg/errors"

func NewCreateUserError(err error) error {
	return errors.Wrap(err, "cannot create user")
}

func NewGrantPrivilegesError(err error) error {
	return errors.Wrap(err, "cannot grant privileges")
}

func NewCheckUserExistsError(err error) error {
	return errors.Wrap(err, "cannot check if user exists")
}

func NewCheckGrantExistsError(err error) error {
	return errors.Wrap(err, "cannot check if grant exists")
}

func NewDeleteUserError(err error) error {
	return errors.Wrap(err, "cannot delete user")
}

func NewCannotDeleteDatabaseError(err error) error {
	return errors.Wrap(err, "cannot delete database")
}

func NewCheckDatabaseExistError(err error) error {
	return errors.Wrap(err, "cannot check if database exists")
}

func NewRevokePrivilegesError(err error) error {
	return errors.Wrap(err, "cannot revoke privileges")
}

func NewCloseConnectionError(err error) error {
	return errors.Wrap(err, "cannot close connection")
}

func NewCheckConnectionError(err error) error {
	return errors.Wrap(err, "cannot check connection")
}

func NewCreateDBError(err error) error {
	return errors.Wrap(err, "cannot create database")
}
