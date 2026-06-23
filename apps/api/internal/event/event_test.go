package event

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangeJSONRoundTrip(t *testing.T) {
	in := Change{Table: "applications", Op: "UPDATE", ID: "abc-123"}
	b, err := json.Marshal(in)
	require.NoError(t, err)
	assert.JSONEq(t, `{"table":"applications","op":"UPDATE","id":"abc-123"}`, string(b))

	var out Change
	require.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, in, out)
}

func TestChangeUnmarshalPartial(t *testing.T) {
	var c Change
	require.NoError(t, json.Unmarshal([]byte(`{"table":"deployments"}`), &c))
	assert.Equal(t, "deployments", c.Table)
	assert.Empty(t, c.Op)
	assert.Empty(t, c.ID)
}
