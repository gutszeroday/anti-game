//go:build windows

package ui

import (
	"strings"
	"time"

	"github.com/guts/antigame/internal/pairing"
)

// showPair, bir kisiye anahtar baglama diyalogunu acar. Uc yerden
// cagriliyor: ilk kurulum, yeni kisi ekleme, anahtar yenileme.
//
// Anahtar yalnizca dogru kod girildikten sonra cagirana veriliyor.
// Yarida birakilan bir eslestirme geride kayit birakmamali.
func showPair(parent uintptr, account string) (secret []byte, counter uint64, ok bool) {
	s, err := pairing.NewSecret()
	if err != nil {
		warn(parent, "antigame", "Anahtar üretilemedi: "+err.Error())
		return nil, 0, false
	}
	uri := pairing.OTPAuthURI(s, account)

	const qrPt = 240
	img, err := pairing.QRImage(uri, qrPt*3)
	if err != nil {
		warn(parent, "antigame", "QR kod üretilemedi: "+err.Error())
		return nil, 0, false
	}

	m, err := newModal(parent, "Anahtar eşleştirme — "+account, 500, 470)
	if err != nil {
		warn(parent, "antigame", "Pencere açılamadı: "+err.Error())
		return nil, 0, false
	}

	qrRect := Rect{m.s(130), m.s(12), m.s(qrPt), m.s(qrPt)}
	m.onPaint = func(hdc uintptr) { drawImage(hdc, img, qrRect) }

	m.label("Google Authenticator veya benzeri bir uygulamada \"QR kodu tara\" ile "+
		"okutun, sonra uygulamada görünen 6 haneli kodu girin.",
		Rect{12, 262, 476, 34})

	m.label("Kod:", Rect{12, 306, 34, 20})
	code := m.edit("", Rect{50, 302, 90, 26}, esNumber|esCenter)

	_, revealID := m.button("Anahtarı göster", Rect{152, 302, 130, 26}, variantSecondary, false)
	copyBtn, copyID := m.button("Kopyala", Rect{292, 302, 90, 26}, variantSecondary, false)
	// Anahtar gosterilmeden kopyalanacak bir sey yok; dugmenin
	// tiklanabilir durmasi yaniltici olurdu.
	enable(copyBtn, false)

	status := m.label("", Rect{12, 338, 476, 70})

	_, okID := m.button("Onayla", Rect{292, 424, 90, 28}, variantPrimary, true)
	_, cancelID := m.button("Vazgeç", Rect{394, 424, 90, 28}, variantSecondary, false)

	m.onCmd = func(id int) {
		switch id {
		case cancelID, idCancel:
			m.close()
		case revealID:
			// Anahtari gormek kapiyi acabilmek demek. Metin akisinda bu
			// adim kayda geciyordu; burada da en azindan ne oldugu
			// acikca soyleniyor.
			setText(status, "DİKKAT: Bu anahtarı gören herkes kapıyı açabilir. "+
				"Arkadaşınıza iletin, kendinizde saklamayın.\n\n"+
				pairing.GroupKey(pairing.EncodeKey(s)))
			enable(copyBtn, true)
		case copyID:
			if err := copySecret(m.hwnd, pairing.GroupKey(pairing.EncodeKey(s))); err != nil {
				setText(status, "Kopyalanamadı: "+err.Error())
				return
			}
			setText(status, "Anahtar panoya kopyalandı. Pano geçmişine (Win+V) "+
				"yazılmadı, ama gönderdikten sonra panoyu temizleyin.")
		case okID, idOK:
			entered := strings.TrimSpace(textOf(code))
			if entered == "" {
				setText(status, "Uygulamada görünen 6 haneli kodu girin.")
				focus(code)
				return
			}
			c, good, msg := pairing.Check(s, entered, time.Now().UTC())
			if !good {
				setText(status, msg)
				focus(code)
				selectAll(code)
				return
			}
			secret, counter, ok = s, c, true
			m.close()
		}
	}

	focus(code)
	m.run(parent)
	return secret, counter, ok
}

// showAssignKey, bir kisiye anahtar atama diyalogunu acar. showPair'den
// farkli olarak onay kodu istemez: anahtar uretilir, QR gosterilir,
// admin "Ekle"ye basinca kisi hemen eklenir. Kod geri istenmedigi icin
// anahtarin dogru aktarildigina dair bir dogrulama yoktur — admin
// anahtari elden ya da guvenli bir kanaldan kendisi paylastigina
// guvenir. Kullanim: yeni kisi ekleme akisi (bkz. people.go addID).
func showAssignKey(parent uintptr, account string) (secret []byte, ok bool) {
	s, err := pairing.NewSecret()
	if err != nil {
		warn(parent, "antigame", "Anahtar üretilemedi: "+err.Error())
		return nil, false
	}
	uri := pairing.OTPAuthURI(s, account)

	const qrPt = 240
	img, err := pairing.QRImage(uri, qrPt*3)
	if err != nil {
		warn(parent, "antigame", "QR kod üretilemedi: "+err.Error())
		return nil, false
	}

	m, err := newModal(parent, "Anahtar oluştur — "+account, 500, 420)
	if err != nil {
		warn(parent, "antigame", "Pencere açılamadı: "+err.Error())
		return nil, false
	}

	qrRect := Rect{m.s(130), m.s(12), m.s(qrPt), m.s(qrPt)}
	m.onPaint = func(hdc uintptr) { drawImage(hdc, img, qrRect) }

	m.label("Google Authenticator veya benzeri bir uygulamada \"QR kodu tara\" ile "+
		"okutun, ya da anahtarı elle paylaşın.", Rect{12, 262, 476, 34})

	status := m.label("", Rect{12, 302, 476, 70})

	_, revealID := m.button("Anahtarı göster", Rect{12, 380, 130, 26}, variantSecondary, false)
	copyBtn, copyID := m.button("Kopyala", Rect{152, 380, 90, 26}, variantSecondary, false)
	// Anahtar gosterilmeden kopyalanacak bir sey yok; dugmenin
	// tiklanabilir durmasi yaniltici olurdu.
	enable(copyBtn, false)

	_, okID := m.button("Ekle", Rect{292, 380, 90, 28}, variantPrimary, true)
	_, cancelID := m.button("Vazgeç", Rect{394, 380, 90, 28}, variantSecondary, false)

	m.onCmd = func(id int) {
		switch id {
		case cancelID, idCancel:
			m.close()
		case revealID:
			setText(status, "DİKKAT: Bu anahtarı gören herkes kapıyı açabilir. "+
				"Arkadaşınıza iletin, kendinizde saklamayın.\n\n"+
				pairing.GroupKey(pairing.EncodeKey(s)))
			enable(copyBtn, true)
		case copyID:
			if err := copySecret(m.hwnd, pairing.GroupKey(pairing.EncodeKey(s))); err != nil {
				setText(status, "Kopyalanamadı: "+err.Error())
				return
			}
			setText(status, "Anahtar panoya kopyalandı. Pano geçmişine (Win+V) "+
				"yazılmadı, ama gönderdikten sonra panoyu temizleyin.")
		case okID, idOK:
			secret, ok = s, true
			m.close()
		}
	}

	m.run(parent)
	return secret, ok
}
