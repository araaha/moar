package m

import (
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/walles/moar/twin"
	"gotest.tools/v3/assert"
)

func TestTwinStyleFromChroma(t *testing.T) {
	// Test that getting exact GenericHeading from base16-snazzy works
	style := twinStyleFromChroma(
		styles.Registry["base16-snazzy"],
		&formatters.TTY16m,
		chroma.GenericHeading,
		true,
	)

	assert.Equal(t,
		*style,
		twin.StyleDefault.
			WithAttr(twin.AttrBold).
			WithForeground(twin.NewColor24Bit(0xe2, 0xe4, 0xe5)))
}
