package m

import (
	"fmt"

	"github.com/walles/moar/m/linenumbers"
	"github.com/walles/moar/m/textstyles"
	"github.com/walles/moar/twin"
)

type overflowState bool

const (
	didFit      overflowState = false
	didOverflow overflowState = true
)

type renderedLine struct {
	inputLine linenumbers.LineNumber

	// If an input line has been wrapped into two, the part on the second line
	// will have a wrapIndex of 1.
	wrapIndex int

	cells []twin.Cell

	// Used for rendering clear-to-end-of-line control sequences:
	// https://en.wikipedia.org/wiki/ANSI_escape_code#EL
	//
	// Ref: https://github.com/walles/moar/issues/106
	trailer twin.Style
}

// Refresh the whole pager display, both contents lines and the status line at
// the bottom
func (p *Pager) redraw(spinner string) overflowState {
	p.screen.Clear()
	p.longestLineLength = 0

	lastUpdatedScreenLineNumber := -1
	var renderedScreenLines [][]twin.Cell
	renderedScreenLines, statusText, overflow := p.renderScreenLines()
	for screenLineNumber, row := range renderedScreenLines {
		lastUpdatedScreenLineNumber = screenLineNumber
		for column, cell := range row {
			p.screen.SetCell(column, lastUpdatedScreenLineNumber, cell)
		}
	}

	// Status line code follows

	eofSpinner := spinner
	if eofSpinner == "" {
		// This happens when we're done
		eofSpinner = ""
	}
	spinnerLine := textstyles.CellsFromString("", _EofMarkerFormat+eofSpinner, nil).Cells
	for column, cell := range spinnerLine {
		p.screen.SetCell(column, lastUpdatedScreenLineNumber+1, cell)
	}

	p.mode.drawFooter(statusText, spinner)

	p.screen.Show()
	return overflow
}

// Render screen lines into an array of lines consisting of Cells.
//
// At most height - 1 lines will be returned, leaving room for one status line.
//
// The lines returned by this method are decorated with horizontal scroll
// markers and line numbers and are ready to be output to the screen.
func (p *Pager) renderScreenLines() (lines [][]twin.Cell, statusText string, overflow overflowState) {
	renderedLines, statusText, overflow := p.renderLines()
	if len(renderedLines) == 0 {
		return
	}

	// Construct the screen lines to return
	screenLines := make([][]twin.Cell, 0, len(renderedLines))
	for _, renderedLine := range renderedLines {
		screenLines = append(screenLines, renderedLine.cells)

		if renderedLine.trailer == twin.StyleDefault {
			continue
		}

		// Fill up with the trailer
		screenWidth, _ := p.screen.Size()
		for len(screenLines[len(screenLines)-1]) < screenWidth {
			screenLines[len(screenLines)-1] =
				append(screenLines[len(screenLines)-1], twin.NewCell(' ', renderedLine.trailer))
		}
	}

	return screenLines, statusText, overflow
}

// Render all lines that should go on the screen.
//
// Returns both the lines and a suitable status text.
//
// The returned lines are display ready, meaning that they come with horizontal
// scroll markers and line numbers as necessary.
//
// The maximum number of lines returned by this method is limited by the screen
// height. If the status line is visible, you'll get at most one less than the
// screen height from this method.
func (p *Pager) renderLines() ([]renderedLine, string, overflowState) {
	wantedLineCount := p.visibleHeight()

	screenOverflow := didFit

	var lineNumber linenumbers.LineNumber
	if p.lineNumber() != nil {
		lineNumber = *p.lineNumber()
	} else {
		// No lines to show, line number doesn't matter, pick anything. But we
		// still want one so that we can get the status text from the reader
		// below.
		lineNumber = linenumbers.LineNumber{}
	}

	if !lineNumber.IsZero() {
		// We're scrolled down, meaning everything is not visible on screen
		screenOverflow = didOverflow
	}

	inputLines, readerOverflow := p.reader.GetLines(lineNumber, wantedLineCount)
	if inputLines.lines == nil {
		// Empty input, empty output
		return []renderedLine{}, inputLines.statusText, didFit
	}
	if readerOverflow == didOverflow {
		// This is not the whole input
		screenOverflow = didOverflow
	}

	allLines := make([]renderedLine, 0)
	for lineIndex, line := range inputLines.lines {

		lineNumber := inputLines.firstLine.NonWrappingAdd(lineIndex)

		rendering, lineOverflow := p.renderLine(line, lineNumber, p.scrollPosition.internalDontTouch)
		if lineOverflow == didOverflow {
			// Everything did not fit
			screenOverflow = didOverflow
		}

		var onScreenLength int
		for i := 0; i < len(rendering); i++ {
			trimmedLen := len(twin.TrimSpaceRight(rendering[i].cells))
			if trimmedLen > onScreenLength {
				onScreenLength = trimmedLen
			}
		}

		// We're trying to find the max length of readable characters to limit
		// the scrolling to right, so we don't go over into the vast emptiness for no reason.
		//
		// The -1 fixed an issue that seemed like an off-by-one where sometimes, when first
		// scrolling completely to the right, the first left scroll did not show the text again.
		displayLength := p.leftColumnZeroBased + onScreenLength - 1

		if displayLength >= p.longestLineLength {
			p.longestLineLength = displayLength
		}

		allLines = append(allLines, rendering...)
	}

	// Find which index in allLines the user wants to see at the top of the
	// screen
	firstVisibleIndex := -1 // Not found
	for index, line := range allLines {
		if p.lineNumber() == nil {
			// Expected zero lines but got some anyway, grab the first one!
			firstVisibleIndex = index
			break
		}
		if line.inputLine == *p.lineNumber() && line.wrapIndex == p.deltaScreenLines() {
			firstVisibleIndex = index
			break
		}
	}
	if firstVisibleIndex == -1 {
		panic(fmt.Errorf("scrollPosition %#v not found in allLines size %d",
			p.scrollPosition, len(allLines)))
	}
	if firstVisibleIndex != 0 {
		// We're scrolled down, meaning everything is not visible on screen
		screenOverflow = didOverflow
	}

	// Drop the lines that should go above the screen
	allLines = allLines[firstVisibleIndex:]

	if len(allLines) <= wantedLineCount {
		// Screen has enough room for everything, return everything
		return allLines, inputLines.statusText, screenOverflow
	}

	screenOverflow = didOverflow
	return allLines[0:wantedLineCount], inputLines.statusText, screenOverflow
}

// Render one input line into one or more screen lines.
//
// The returned line is display ready, meaning that it comes with horizontal
// scroll markers and line number as necessary.
//
// lineNumber and numberPrefixLength are required for knowing how much to
// indent, and to (optionally) render the line number.
func (p *Pager) renderLine(line *Line, lineNumber linenumbers.LineNumber, scrollPosition scrollPositionInternal) ([]renderedLine, overflowState) {
	highlighted := line.HighlightedTokens(p.linePrefix, p.searchPattern, &lineNumber)
	var wrapped [][]twin.Cell
	overflow := didFit
	if p.WrapLongLines {
		width, _ := p.screen.Size()
		wrapped = wrapLine(width-numberPrefixLength(p, scrollPosition), highlighted.Cells)
	} else {
		// All on one line
		wrapped = [][]twin.Cell{highlighted.Cells}
	}

	if len(wrapped) > 1 {
		overflow = didOverflow
	}

	rendered := make([]renderedLine, 0)
	for wrapIndex, inputLinePart := range wrapped {
		visibleLineNumber := &lineNumber
		if wrapIndex > 0 {
			visibleLineNumber = nil
		}

		decorated, localOverflow := p.decorateLine(visibleLineNumber, inputLinePart, scrollPosition)
		if localOverflow == didOverflow {
			overflow = didOverflow
		}

		rendered = append(rendered, renderedLine{
			inputLine: lineNumber,
			wrapIndex: wrapIndex,
			cells:     decorated,
		})
	}

	if highlighted.Trailer != twin.StyleDefault {
		// In the presence of wrapping, add the trailer to the last of the wrap
		// lines only. This matches what both iTerm and the macOS Terminal does.
		rendered[len(rendered)-1].trailer = highlighted.Trailer
	}

	return rendered, overflow
}

// Take a rendered line and decorate as needed:
// * Line number, or leading whitespace for wrapped lines
// * Scroll left indicator
// * Scroll right indicator
func (p *Pager) decorateLine(lineNumberToShow *linenumbers.LineNumber, contents []twin.Cell, scrollPosition scrollPositionInternal) ([]twin.Cell, overflowState) {
	width, _ := p.screen.Size()
	newLine := make([]twin.Cell, 0, width)
	numberPrefixLength := numberPrefixLength(p, scrollPosition)
	newLine = append(newLine, createLinePrefix(lineNumberToShow, numberPrefixLength)...)
	overflow := didFit

	startColumn := p.leftColumnZeroBased
	if startColumn < len(contents) {
		endColumn := p.leftColumnZeroBased + (width - numberPrefixLength)
		if endColumn > len(contents) {
			endColumn = len(contents)
		}

		newLine = append(newLine, contents[startColumn:endColumn]...)
	}

	return newLine, overflow
}

// Generate a line number prefix of the given length.
//
// Can be empty or all-whitespace depending on parameters.
func createLinePrefix(lineNumber *linenumbers.LineNumber, numberPrefixLength int) []twin.Cell {
	if numberPrefixLength == 0 {
		return []twin.Cell{}
	}

	lineNumberPrefix := make([]twin.Cell, 0, numberPrefixLength)
	if lineNumber == nil {
		for len(lineNumberPrefix) < numberPrefixLength {
			lineNumberPrefix = append(lineNumberPrefix, twin.Cell{Rune: ' '})
		}
		return lineNumberPrefix
	}

	lineNumberString := fmt.Sprintf("%*s ", numberPrefixLength-1, lineNumber.Format())
	if len(lineNumberString) > numberPrefixLength {
		panic(fmt.Errorf(
			"lineNumberString <%s> longer than numberPrefixLength %d",
			lineNumberString, numberPrefixLength))
	}

	for column, digit := range lineNumberString {
		if column >= numberPrefixLength {
			break
		}

		lineNumberPrefix = append(lineNumberPrefix, twin.NewCell(digit, lineNumbersStyle))
	}

	return lineNumberPrefix
}
