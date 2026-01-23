# DarkWatch - Interactive Threat Intelligence Scraper

DarkWatch, Dark Web (.onion) ve Surface Web kaynaklarını izleyen, otomatik olarak tehdit analizi yapan ve ilişkisel veri görselleştirmesi sunan Go tabanlı bir Siber Tehdit İstihbaratı (CTI) aracıdır.

## Özellikler


- **Otomatik Tarama & Deep Scan:** Belirlenen hedefleri ve alt linklerini otomatik tarar.
- **Tor Entegrasyonu:** `.onion` sitelerine güvenli erişim için dahili Tor proxy kullanır.
- **Akıllı Tehdit Analizi:**
  - Anahtar kelime bazlı risk skorlaması (Ransomware, Leak, vb.).
  - MITRE ATT&CK çerçevesi ile uyumlu etiketleme.
  - Varlık Tespiti (Bitcoin, Email, PGP, IP).
- **İlişki Analizi (Link Analysis):** Siteler arası linkleri ve ortak kullanılan varlıkları (cüzdan, email vb.) görselleştirerek suç ağlarını ortaya çıkarır.
- **Raporlama:** Analiz sonuçlarını JSON ve PDF formatında dışa aktarma.
- **Admin Paneli:** Risk skorlarını manuel düzenleme ve doğrulama imkanı.

## Teknoloji Yığını

- **Backend:** Go (Golang) 1.25
- **Veritabanı:** PostgreSQL 15
- **Ağ:** Tor (Socks5 Proxy), Docker Networks
- **Frontend:** HTML5, CSS3, Chart.js, Vis.js (Graph Visualization)
- **Konteynerizasyon:** Docker & Docker Compose

## Kurulum ve Çalıştırma

Bu proje Docker ile tamamen konteynerize edilmiştir. Çalıştırmak için sisteminizde Docker'ın yüklü olması yeterlidir.

### 1. Projeyi Başlatın
Terminali proje dizininde açın ve şu komutu girin:

```bash
docker-compose up -d --build