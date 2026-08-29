# antigame

![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)
![Platform](https://img.shields.io/badge/platform-Windows-0078D6?logo=windows&logoColor=white)
![License](https://img.shields.io/badge/license-unlicensed-lightgrey)

Windows için oyun süresi takibi ve MFA (TOTP) tabanlı kapı sistemi. Belirlenen oyunlar açıldığında süre kaydedilir; süre aşıldığında oyun kapıda durdurulur ve devam etmek için yetkili bir kişinin TOTP kodu gerekir.

<!-- ![screenshot](docs/screenshot.png) -->

## Özellikler

- **İzleyici (watch)** — çalışan pencereleri periyodik tarar, kapıdaki oyunun süresini biriktirir, boşta kalma süresini hesaba katar.
- **MFA kapısı (gate)** — süre dolunca oyunu durdurur, kayıtlı kişilerden birinin TOTP kodunu ister; kod doğrulanınca oturum yeniden açılır.
- **Kişi yönetimi (people)** — kapıyı açabilecek kişileri ve TOTP anahtarlarını (QR ile) yönetir.
- **Oyun listesi (list)** — kapıda durdurulacak exe/launcher listesini düzenler.
- **Haftalık rapor (report)** — kullanım geçmişini tarayıcıda HTML rapor olarak açar.
- **Telegram entegrasyonu** — eşleştirme koduyla onaylanan sohbetlere `/durum` ve kapanış bildirimleri gönderir; ağ çağrıları izleyici döngüsünden tamamen bağımsız çalışır.
- **Sistem tepsisi + başlangıç görevi** — oturum açılışında arka planda otomatik başlar.

## Kurulum ve Kullanım

```
antigame setup              Kurulum sihirbazı (MFA eşleştirme)
antigame watch               İzleyiciyi başlat (zamanlanmış görev çalıştırır)
antigame gate --app <ad>     Kod giriş penceresi
antigame gate --manual       Oyun açmadan kod gir
antigame list                Oyun listesini görüntüle / düzenle
antigame people              Kapıyı açabilen kişileri yönet
antigame report               Haftalık raporu tarayıcıda aç
antigame autostart            Başlangıca ekle / çıkar
antigame uninstall             Kodla doğrulayıp kaldır
```

Argümansız çalıştırıldığında (çift tıklayarak) grafik arayüz açılır; arayüz açılamazsa metin menüsüne düşülür.

### Telegram bot komutları (onaylı sohbetten)

- `/durum` — haftalık özet
- `/help` — komut listesi
- `/kapanis_bildirimi` — izleyici kapanınca bildirim aç/kapat

## Geliştirme

```
go build ./...
go test ./...
```

Windows kaynakları (`.syso`) ve ikon repoda tutulur; her mimarinin (`amd64`, `arm64`, `386`) kendi manifest kaynağı gerekir, derlemeden önce ayrıca üretilmesi gerekmez.

## Yapılandırma

Ayarlar kullanıcı veri klasöründeki `config.json` içinde tutulur (kişi TOTP anahtarları ayrı bir vault'ta). Bu dosyalar repoya dahil değildir.
