package prpc

import (
	"testing"

	"platoIM/common/config"

	ptrace "platoIM/common/prpc/trace"

	"github.com/stretchr/testify/assert"
)

func TestNewPClient(t *testing.T) {
	config.Init("../../plato.yaml")
	ptrace.StartAgent()
	defer ptrace.StopAgent()

	_, err := NewPClient("plato_server")
	assert.NoError(t, err)
}
