//go:build !windows

package tray

import _ "embed"

// Иконки генерирует tools/genicons. На Linux трей принимает PNG.

var (
	//go:embed icons/off.png
	iconOff []byte
	//go:embed icons/on.png
	iconOn []byte
	//go:embed icons/lost.png
	iconLost []byte
)

var icons = map[State][]byte{Off: iconOff, On: iconOn, Lost: iconLost}
