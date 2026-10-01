# Swappy

Tauschplattform für iOS: Gegenstände tauschen statt kaufen. Kein Preis, kein Geld.

- `backend/`: Go + Postgres (HTTP-API, Tauschlogik, Migrationen, Seed)
- `frontend/`: SwiftUI-App (iOS 17+), Projekt per XcodeGen

## Lokal starten

```bash
# 1. Datenbank
docker compose up -d db

# 2. Backend: Schema + Testdaten, dann Server mit Dev-Login
cd backend
go run ./cmd/swappy-backend reset
SWAPPY_DEV_LOGIN=true go run ./cmd/swappy-backend serve     # http://localhost:8080
```

> Nach `reset` den laufenden Server **neu starten**. Er hält vorbereitete
> Statements und Typ-IDs des alten Schemas im Cache.

```bash
# 3. App
cd frontend
xcodegen generate          # erzeugt Swappy.xcodeproj aus project.yml
open Swappy.xcodeproj      # Scheme "Swappy", Simulator wählen, ⌘R
```

In der App auf **„Ich bin Dev – ohne Login testen“** tippen und `anna` oder
`ben` wählen, oder ein neues Testprofil anlegen. Auf einem echten iPhone im
Dev-Login unter „Backend“ die IP-Adresse des Macs eintragen.

### Dev-Login

`SWAPPY_DEV_LOGIN=true` schaltet `POST /v1/dev/login` frei: Token für jeden
Anzeigenamen, ohne Passwort. **Nur lokal.** Ohne die Variable existiert der
Endpunkt nicht, und der Server verlangt `SWAPPY_TOKEN_SECRET`.

## Tests

```bash
cd backend && go test -race ./...          # braucht die DB aus compose

cd frontend && xcodebuild test -project Swappy.xcodeproj -scheme Swappy \
  -destination 'platform=iOS Simulator,name=iPhone 17'
```

Die UI-Tests (`SwappyUITests`) spielen einen kompletten Tausch gegen das lokale
Backend durch. Vorher `reset` ausführen und den Server neu starten.

## API (Kurzüberblick)

Alle Endpunkte unter `/v1` verlangen `Authorization: Bearer <token>`, außer
Dev-Login und Bilder. Fehler kommen als `{"error":{"code","message"}}`.

| Bereich | Endpunkte |
|---|---|
| Ich | `GET/PUT/DELETE /me`, `GET /me/items`, `GET /me/blocks` |
| Items | `GET/POST /items`, `GET/PUT/DELETE /items/{id}`, `POST /items/{id}/publish`, `POST /items/{id}/images` (JPEG-Body), `DELETE /items/{id}/images/{imageID}` |
| Angebote | `GET/POST /offers`, `GET /offers/{id}`, `POST /offers/{id}/accept\|decline\|cancel\|confirm` |
| Nutzer | `GET /users/{id}`, `POST/DELETE /users/{id}/block`, `POST /reports` |
| Aktivität | `GET /notifications`, `POST /notifications/read` |
| Bilder | `GET /images/{name}` |
