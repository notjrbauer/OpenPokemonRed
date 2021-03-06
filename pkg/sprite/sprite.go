package sprite

import (
	"fmt"
	"image/png"
	"pokered/pkg/data/sprdata"
	"pokered/pkg/data/worldmap/object"
	"pokered/pkg/joypad"
	"pokered/pkg/screen"
	"pokered/pkg/store"
	"pokered/pkg/text"
	"pokered/pkg/util"
	"pokered/pkg/world"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Movement status
const (
	Uninitialized byte = iota
	OK
	Delay
	Movement
)

// Player sprite state
const (
	Normal uint = iota
	Cycling
	Seel
)

func InitMapSprites() {
	for i := 1; i < 16; i++ {
		store.SpriteData[i] = nil
	}
	sprites := world.CurWorld.Object.Sprites
	for i, s := range sprites {
		hidden := world.CurWorld.Object.HS[i+1]
		addSprite(s.ID, s.XCoord, s.YCoord, s.MovementBytes, s.TextID, hidden)
	}
}

// AddSprite add sprite into SpriteData
func addSprite(id sprdata.SpriteID, x, y util.Coord, movementBytes [2]byte, textID int, hidden bool) {
	imgs := make([]*ebiten.Image, 10)
	for i := 0; i < 10; i++ {
		name := id.String()
		path := fmt.Sprintf("/%s_%d.png", name, i)

		f, err := store.FS.Open(path)
		if err != nil {
			if i > 0 {
				break
			}
			f, err = store.FS.Open(fmt.Sprintf("/%s.png", name))
			if err != nil {
				break
			}
		}

		defer f.Close()

		img, _ := png.Decode(f)
		imgs[i] = ebiten.NewImageFromImage(img)
	}

	p := store.SpriteData[0]
	n := store.NumSprites()
	s := &store.Sprite{
		ScreenXPixel:  16 * (x - p.MapXCoord + 4),
		ScreenYPixel:  16*(y+4-p.MapYCoord) - 4,
		MapXCoord:     x,
		MapYCoord:     y,
		MovementBytes: movementBytes,
		VRAM: store.SpriteImage{
			Index:  1,
			Images: imgs,
		},
		TextID: textID,
		Hidden: hidden,
	}
	store.SpriteData[n] = s
}

// UpdateSprites update sprite data
func UpdateSprites() {
	for offset := range store.SpriteData {
		if store.IsInvalidSprite(uint(offset)) {
			break
		}
		if offset == 0 {
			UpdatePlayerSprite()
			continue
		}
		UpdateNPCSprite(uint(offset))
	}
}

// UpdateSpriteImage update sprite image index
func UpdateSpriteImage(offset uint) {
	s := store.SpriteData[offset]
	if s == nil {
		return
	}

	length := s.VRAM.Length()
	if length == 1 {
		s.VRAM.Index = 0
		return
	}
	if length == 4 {
		updateSpriteImage4(offset)
		return
	}

	index := 0
	switch s.AnimationCounter() + uint(s.Direction) {
	// down
	case 0, 3:
		index = 1
	case 1, 2:
		index = 0
		if s.RightHand {
			index = 2
		}

	// up
	case 4:
		index = 4
	case 5:
		index = 3
		if s.RightHand {
			index = 5
		}
	case 6:
		index = 3
		if s.RightHand {
			index = 5
		}
	case 7:
		index = 4

	case 8, 11:
		index = 6
	case 9, 10:
		index = 7

	case 12, 15:
		index = 8
	case 13, 14:
		index = 9
	}

	fontLoaded := text.FontLoaded()
	if fontLoaded {
		switch s.AnimationCounter() + uint(s.Direction) {
		case 0, 1, 2, 3:
			index = 1
		case 4, 5, 6, 7:
			index = 4
		case 8, 9, 10, 11:
			index = 6
		case 12, 13, 14, 15:
			index = 8
		}
	}

	s.VRAM.Index = index
}

func updateSpriteImage4(offset uint) {
	s := store.SpriteData[offset]
	index := 0
	switch s.AnimationCounter() + uint(s.Direction) {

	// down
	case 0, 3:
		index = 0
	case 1, 2:
		index = 0
		if s.RightHand {
			index = 2
		}

	// up
	case 4, 7:
		index = 1
	case 5, 6:
		index = 1
		if s.RightHand {
			index = 5
		}

	case 8, 9, 10, 11:
		index = 2

	case 12, 13, 14, 15:
		index = 3
	}
	s.VRAM.Index = index
}

// DisableSprite hide sprite
func DisableSprite(offset uint) {
	s := store.SpriteData[offset]
	s.VRAM.Index = -1
}

// MoveSpriteForcely move sprite by movement data forcely
// set wNPCMovementDirections
func MoveSpriteForcely(offset uint, movement []byte) {
	copy(NPCMovementDirections, movement)
	store.Flag.D730.IsNPCScripted = false
	joypad.JoyIgnore = joypad.ByteToInput(0xff)
}

// drawSprite
func drawSprite(target *ebiten.Image, offset uint) {
	s := store.SpriteData[offset]
	UpdateSpriteImage(offset)
	util.DrawImagePixel(target, s.VRAM.Images[s.VRAM.Index], s.ScreenXPixel, s.ScreenYPixel)
}

// VBlank script executed in VBlank
func VBlank() {
	if world.CurWorld == nil {
		return
	}

	if !world.CurWorld.Object.Initialized {
		InitMapSprites()
		world.CurWorld.Object.Initialized = true
	}

	l := util.NewImage()
	for i, s := range store.SpriteData {
		if store.IsInvalidSprite(uint(i)) {
			break
		}
		if s.VRAM.Index < 0 {
			continue
		}
		drawSprite(l, uint(i))
	}
	screen.AddLayer("sprite", screen.Sprite, l, 0, 0)
}

// GetFrontSpriteOrSign check sprite or sign is in front of player
func GetFrontSpriteOrSign(offset int) int {
	s := store.SpriteData[offset]
	if s == nil {
		return -1
	}

	xCoord, yCoord, direction := s.MapXCoord, s.MapYCoord, s.Direction
	switch direction {
	case util.Up:
		yCoord--
	case util.Down:
		yCoord++
	case util.Left:
		xCoord--
	case util.Right:
		xCoord++
	}

	signs := world.CurWorld.Object.Signs
	for _, sign := range signs {
		if xCoord == sign.XCoord && yCoord == sign.YCoord {
			return sign.TextID
		}
	}

	for i, npc := range store.SpriteData {
		if i == offset {
			continue
		}

		if npc == nil {
			return -1
		}

		if npc.Hidden {
			continue
		}

		if xCoord == npc.MapXCoord && yCoord == npc.MapYCoord {
			return i
		}
	}

	return -1
}

func GetFrontHiddentObject() (object.HiddenObject, bool) {
	p := store.SpriteData[0]

	xCoord, yCoord, direction := p.MapXCoord, p.MapYCoord, p.Direction
	switch direction {
	case util.Up:
		yCoord--
	case util.Down:
		yCoord++
	case util.Left:
		xCoord--
	case util.Right:
		xCoord++
	}

	objects := world.CurWorld.Object.HO
	for _, o := range objects {
		if xCoord == o.XCoord && yCoord == o.YCoord {
			return o, true
		}
	}

	return object.HiddenObject{}, false
}
