package installer

import "testing"

func TestDMGTransportRevisionBindsOnlyDiskImages(t *testing.T) {
	pins, err := DefaultArchivePins(NativePlatform{Context: Context{OS: "darwin", Arch: "arm64"}})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for id, pin := range pins {
		if pin.Format != "dmg" {
			if pin.DMGRevision != 0 {
				t.Fatal("revision escaped DMG transport", id)
			}
			continue
		}
		count++
		if pin.DMGRevision != 1 {
			t.Fatal("current image lacks corrected transport", id)
		}
		for _, revision := range []int{-1, 0, 2} {
			pin.DMGRevision = revision
			if pin.Validate() == nil {
				t.Fatal("unknown transport accepted", revision)
			}
		}
		pin.DMGRevision, pin.Format, pin.AppBundle = 1, "zip", ""
		if pin.Validate() == nil {
			t.Fatal("DMG transport accepted for ZIP")
		}
	}
	if count == 0 {
		t.Fatal("fixture exercised no disk-image pins")
	}
}
