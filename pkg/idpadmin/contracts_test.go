package idpadmin

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCursorPageRequestNormalized(t *testing.T) {
	require.Equal(t, DefaultPageSize, (CursorPageRequest{}).Normalized().Limit)
	require.Equal(t, MaxPageSize, (CursorPageRequest{Limit: MaxPageSize + 1}).Normalized().Limit)
	require.Equal(t, 10, (CursorPageRequest{Limit: 10}).Normalized().Limit)
}

func TestSafeDTOsDoNotExposeSecretMaterial(t *testing.T) {
	require.NotContains(t, fieldNames(ClientDetail{}), "SecretHash")
	require.NotContains(t, fieldNames(SigningKeyRow{}), "PrivateKeyPEM")
	require.NotContains(t, fieldNames(UserDetail{}), "PasswordHash")
}

func fieldNames(value any) []string {
	typ := reflect.TypeOf(value)
	names := make([]string, 0, typ.NumField())
	for index := range typ.NumField() {
		names = append(names, typ.Field(index).Name)
	}
	return names
}
