// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package errors

import "fmt"

func NewCreateUserError(err error) error {
	return fmt.Errorf("cannot create user: %w", err)
}

func NewGrantPrivilegesError(err error) error {
	return fmt.Errorf("cannot grant privileges: %w", err)
}

func NewCheckUserExistsError(err error) error {
	return fmt.Errorf("cannot check if user exists: %w", err)
}

func NewCheckGrantExistsError(err error) error {
	return fmt.Errorf("cannot check if grant exists: %w", err)
}

func NewDeleteUserError(err error) error {
	return fmt.Errorf("cannot delete user: %w", err)
}

func NewCannotDeleteDatabaseError(err error) error {
	return fmt.Errorf("cannot delete database: %w", err)
}

func NewCheckDatabaseExistError(err error) error {
	return fmt.Errorf("cannot check if database exists: %w", err)
}

func NewRevokePrivilegesError(err error) error {
	return fmt.Errorf("cannot revoke privileges: %w", err)
}

func NewCloseConnectionError(err error) error {
	return fmt.Errorf("cannot close connection: %w", err)
}

func NewCheckConnectionError(err error) error {
	return fmt.Errorf("cannot check connection: %w", err)
}

func NewCreateDBError(err error) error {
	return fmt.Errorf("cannot create database: %w", err)
}
