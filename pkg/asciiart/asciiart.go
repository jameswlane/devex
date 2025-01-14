package asciiart

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ASCII art content
var asciiArt = `
DDDDDDDDDDDDD                                                 EEEEEEEEEEEEEEEEEEEEEE
D::::::::::::DDD                                              E::::::::::::::::::::E
D:::::::::::::::DD                                            E::::::::::::::::::::E
DDD:::::DDDDD:::::D                                           EE::::::EEEEEEEEE::::E
  D:::::D    D:::::D     eeeeeeeeeeee  vvvvvvv           vvvvvvvE:::::E       EEEEEExxxxxxx      xxxxxxx
  D:::::D     D:::::D  ee::::::::::::ee v:::::v         v:::::v E:::::E              x:::::x    x:::::x
  D:::::D     D:::::D e::::::eeeee:::::eev:::::v       v:::::v  E::::::EEEEEEEEEE     x:::::x  x:::::x
  D:::::D     D:::::De::::::e     e:::::e v:::::v     v:::::v   E:::::::::::::::E      x:::::xx:::::x
  D:::::D     D:::::De:::::::eeeee::::::e  v:::::v   v:::::v    E:::::::::::::::E       x::::::::::x
  D:::::D     D:::::De:::::::::::::::::e    v:::::v v:::::v     E::::::EEEEEEEEEE        x::::::::x
  D:::::D     D:::::De::::::eeeeeeeeeee      v:::::v:::::v      E:::::E                  x::::::::x
  D:::::D    D:::::D e:::::::e                v:::::::::v       E:::::E       EEEEEE    x::::::::::x
DDD:::::DDDDD:::::D  e::::::::e                v:::::::v      EE::::::EEEEEEEE:::::E   x:::::xx:::::x
D:::::::::::::::DD    e::::::::eeeeeeee         v:::::v       E::::::::::::::::::::E  x:::::x  x:::::x
D::::::::::::DDD       ee:::::::::::::e          v:::v        E::::::::::::::::::::E x:::::x    x:::::x
DDDDDDDDDDDDD            eeeeeeeeeeeeee           vvv         EEEEEEEEEEEEEEEEEEEEEExxxxxxx      xxxxxxx
`

// Color gradient for ASCII art
var colors = []lipgloss.Color{
	lipgloss.Color("#00FFFF"), // Cyan
	lipgloss.Color("#5F87FF"), // Light Blue
	lipgloss.Color("#5FD7FF"), // Sky Blue
	lipgloss.Color("#5F87D7"), // Dodger Blue
	lipgloss.Color("#005FFF"), // Deep Sky Blue
	lipgloss.Color("#1F3FBF"), // Cornflower Blue
	lipgloss.Color("#0000FF"), // Royal Blue
}

// RenderArt renders the ASCII art with a color gradient.
func RenderArt() {
	lines := strings.Split(asciiArt, "\n")

	for i, line := range lines {
		renderedLine := applyGradient(line, i)
		fmt.Println(renderedLine)
	}
}

// applyGradient applies a gradient to a single line of ASCII art.
func applyGradient(line string, index int) string {
	colorIndex := index % len(colors)
	style := lipgloss.NewStyle().Foreground(colors[colorIndex])
	return style.Render(line)
}
