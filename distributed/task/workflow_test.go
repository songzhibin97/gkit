package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewChain(t *testing.T) {
	t.Parallel()

	task1 := Signature{
		Name: "foo",
		Args: []Arg{
			{
				Type:  "float64",
				Value: interface{}(1),
			},
			{
				Type:  "float64",
				Value: interface{}(1),
			},
		},
	}

	task2 := Signature{
		Name: "bar",
		Args: []Arg{
			{
				Type:  "float64",
				Value: interface{}(5),
			},
			{
				Type:  "float64",
				Value: interface{}(6),
			},
		},
	}

	task3 := Signature{
		Name: "qux",
		Args: []Arg{
			{
				Type:  "float64",
				Value: interface{}(4),
			},
		},
	}

	chain, err := NewChain("", &task1, &task2, &task3)
	if err != nil {
		t.Fatal(err)
	}

	firstTask := chain.Tasks[0]

	assert.Equal(t, "foo", firstTask.Name)
	assert.Equal(t, "bar", firstTask.CallbackOnSuccess[0].Name)
	assert.Equal(t, "qux", firstTask.CallbackOnSuccess[0].CallbackOnSuccess[0].Name)
}
