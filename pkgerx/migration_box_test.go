package pkgerx

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/galaxyed/x/sqlcon/dockertest"

	"github.com/markbates/pkger"
	"github.com/stretchr/testify/require"

	"github.com/galaxyed/x/logrusx"
)

var testData = pkger.Dir("github.com/galaxyed/x:/pkgerx/testdata")

func TestMigrationBoxTemplating(t *testing.T) {
	templateVals := map[string]interface{}{
		"tableName": "test_table_name",
	}

	expectedMigration, err := ioutil.ReadFile(filepath.Join("testdata", "0_sql_create_tablename_template.expected.sql"))
	require.NoError(t, err)

	c := dockertest.ConnectToTestCockroachDBPop(t)

	mb, err := NewMigrationBox(testData, c, logrusx.New("", ""), WithTemplateValues(templateVals), WithMigrationContentMiddleware(func(content string, err error) (string, error) {
		require.NoError(t, err)
		assert.Equal(t, string(expectedMigration), content)

		return content, err
	}))
	require.NoError(t, err)

	err = mb.Up()
	require.NoError(t, err)
}
