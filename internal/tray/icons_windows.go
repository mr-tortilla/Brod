package tray

import _ "embed"

// Иконки генерирует tools/genicons. На Windows трей принимает ICO.

var (
	//go:embed icons/off.ico
	iconOff []byte
	//go:embed icons/on.ico
	iconOn []byte
	//go:embed icons/lost.ico
	iconLost []byte
)

var icons = map[State][]byte{Off: iconOff, On: iconOn, Lost: iconLost}
