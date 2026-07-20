package mcpclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeJSON(t *testing.T) {
	t.Run("simple object", func(t *testing.T) {
		input := []byte(`{"name":"test","value":42}`)
		result, err := DecodeJSON(input)
		require.NoError(t, err)

		om, ok := result.(OrderedMap)
		require.True(t, ok)
		assert.Equal(t, 2, len(om))
		assert.Equal(t, "name", om[0].Key)
		assert.Equal(t, "test", om[0].Value)
	})

	t.Run("array", func(t *testing.T) {
		input := []byte(`[1,2,3]`)
		result, err := DecodeJSON(input)
		require.NoError(t, err)

		arr, ok := result.([]interface{})
		require.True(t, ok)
		assert.Equal(t, 3, len(arr))
	})

	t.Run("nested object", func(t *testing.T) {
		input := []byte(`{"outer":{"inner":"value"}}`)
		result, err := DecodeJSON(input)
		require.NoError(t, err)

		om, ok := result.(OrderedMap)
		require.True(t, ok)
		inner, ok := om[0].Value.(OrderedMap)
		require.True(t, ok)
		assert.Equal(t, "value", inner[0].Value)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		input := []byte(`not json`)
		_, err := DecodeJSON(input)
		assert.Error(t, err)
	})
}

func TestOrderedKeys(t *testing.T) {
	obj := map[string]interface{}{
		"z": 1,
		"a": 2,
		"m": 3,
	}
	keys := OrderedKeys(obj)
	assert.Equal(t, []string{"a", "m", "z"}, keys)
}

func TestExtractDataArray(t *testing.T) {
	t.Run("map with data key", func(t *testing.T) {
		data := map[string]interface{}{
			"data": []interface{}{"item1", "item2"},
		}
		result := ExtractDataArray(data)
		arr, ok := result.([]interface{})
		require.True(t, ok)
		assert.Equal(t, 2, len(arr))
	})

	t.Run("map without data key", func(t *testing.T) {
		data := map[string]interface{}{
			"other": "value",
		}
		result := ExtractDataArray(data)
		_, ok := result.(map[string]interface{})
		assert.True(t, ok)
	})

	t.Run("ordered map with data key", func(t *testing.T) {
		data := OrderedMap{
			{Key: "data", Value: []interface{}{"item1"}},
		}
		result := ExtractDataArray(data)
		arr, ok := result.([]interface{})
		require.True(t, ok)
		assert.Equal(t, 1, len(arr))
	})
}

func TestParseArrayArg(t *testing.T) {
	t.Run("JSON array", func(t *testing.T) {
		result := ParseArrayArg(`["a","b","c"]`, nil)
		assert.Equal(t, 3, len(result))
		assert.Equal(t, "a", result[0])
	})

	t.Run("comma separated", func(t *testing.T) {
		result := ParseArrayArg("a,b,c", nil)
		assert.Equal(t, 3, len(result))
		assert.Equal(t, "a", result[0])
	})

	t.Run("append to existing", func(t *testing.T) {
		existing := []interface{}{"a"}
		result := ParseArrayArg("b,c", existing)
		assert.Equal(t, 3, len(result))
		assert.Equal(t, "a", result[0])
		assert.Equal(t, "b", result[1])
	})
}

func TestOrderedMap(t *testing.T) {
	om := OrderedMap{
		{Key: "name", Value: "test"},
		{Key: "count", Value: 42},
	}

	t.Run("keys", func(t *testing.T) {
		keys := om.Keys()
		assert.Equal(t, []string{"name", "count"}, keys)
	})

	t.Run("get existing key", func(t *testing.T) {
		val := om.Get("name")
		assert.Equal(t, "test", val)
	})

	t.Run("get missing key", func(t *testing.T) {
		val := om.Get("missing")
		assert.Nil(t, val)
	})
}
