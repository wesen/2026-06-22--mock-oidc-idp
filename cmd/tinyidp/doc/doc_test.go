package doc

import (
	"strings"
	"testing"

	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/go-go-golems/glazed/pkg/help/model"
	"github.com/stretchr/testify/require"
)

func TestLocalDevelopmentTutorialIsDiscoverable(t *testing.T) {
	helpSystem := help.NewHelpSystem()
	require.NoError(t, AddDocToHelpSystem(helpSystem))

	section, err := helpSystem.GetSectionWithSlug("tutorial-local-development-apps")
	require.NoError(t, err)
	require.Equal(t, model.SectionTutorial, section.SectionType)
	require.True(t, section.IsTopLevel)
	require.Contains(t, section.Commands, "serve-production")
	require.Contains(t, section.Flags, "listener-mode")
	require.True(t, strings.Contains(section.Content, "devctl --profile my-app smoke"))
	require.True(t, strings.Contains(section.Content, "tiny-idp/dev/_shared/caddy-local/pki-storage"))
}
