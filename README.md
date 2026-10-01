# Swappy 🔄

**Swappy** ist ein moderner Tausch-Marktplatz (Barter Platform) für direkte Gegenstand-zu-Gegenstand-Tauschgeschäfte (z. B. *PS5 gegen Gaming-PC*, *iPhone gegen Smartphone*, *Fahrrad gegen E-Scooter*) mit optionalem Wertausgleich in Euro.

Das Projekt umfasst ein robustes, schnelles **Go-Backend** mit SQLite (pure Go, CGO-frei) und ein **Swift-Frontend/Client SDK** mit Testrunner zur End-to-End-Verifikation aller Tauschabläufe.

---

## 🚀 Kernfunktionen & Architektur

### 1. Benutzer & Profile
- **Registrierung & Authentifizierung**: JWT-basierte Authentifizierung mit `bcrypt`-Passwort-Hashing.
- **Benutzerprofil**: Profilbild, Benutzername, Vorname, Standort (Stadt, Koordinaten), Bio.
- **Vertrauensmerkmale**: Anzahl erfolgreicher Tauschgeschäfte (`completed_trades_count`), Sterne-Durchschnitt (`rating_average`), Gesamtanzahl an Bewertungen (`rating_count`) und Verifizierungsstatus.
- **Sicherheitsfunktionen**: Meldesystem (`reports`) und Blockiersystem (`blocks`).

### 2. Objekte / Posts
- **Felder**: Titel, Beschreibung, Kategorie, Zustand (`Neu`, `Wie neu`, `Sehr gut`, `Gut`, `Akzeptabel`), geschätzter Wert, Tauschinteressen (`trade_interests`), Bilder, Standort.
- **Statusverwaltung**:
  - 🟢 `available` (Verfügbar auf dem Marktplatz)
  - 🟡 `requested` (In aktivem, vereinbartem Tauschprozess reserviert)
  - 🔴 `traded` (Erfolgreich getauscht)
  - ⚪ `deactivated` (Vom Benutzer oder Admin deaktiviert)
- **Favoriten (Merkliste)**: Speichern von interessanten Objekten mit Benachrichtigungen.

### 3. Wichtige Tauschregel (Regel 4) 🛡️
> **Ein Benutzer kann erst eine Tauschanfrage senden, wenn er mindestens ein eigenes Objekt veröffentlicht hat.**
- Das Backend prüft bei `POST /api/swaps` automatisch, ob der Anfragende mindestens ein aktives Objekt im Status `available` besitzt.
- Hat der Nutzer kein eigenes Objekt, wird die Anfrage strikt mit `400 Bad Request` abgewiesen:
  `"you must have at least one active item published before you can propose a swap"`

### 4. Tauschsystem & Verhandlung
- **Tauschangebot**: Zielobjekt + eines oder mehrere eigene angebotene Objekte + optionaler Wertausgleich (`cash_adjustment`, z. B. `+50 €`) + Nachricht + Übergabemethode (`personal` oder `shipping`).
- **Gegenangebot (Counter-Offer)**: Der Empfänger kann die angebotenen Objekte oder den Wertausgleich anpassen (z. B. `+100 €`).
- **Annehmen / Ablehnen**: Nach Einigung wechselt der Status zu `agreed` (🔄 *Tausch vereinbart*), und die beteiligten Objekte werden auf `requested` (🟡) gesetzt.
- **Tausch abschließen**: Beide Parteien bestätigen die Übergabe (`POST /api/swaps/{id}/complete`).
  - Der Tausch wechselt auf `completed`.
  - Alle beteiligten Objekte werden automatisch als `traded` (🔴) markiert.
  - Der Zähler `completed_trades_count` beider Benutzer wird um 1 erhöht.

### 5. Chat & Nachrichten
- Chat-Thread für jeden Tauschvorschlag.
- Systemnachrichten bei Statusänderungen (Angebot gesendet, Gegenangebot, Tausch vereinbart, Tausch abgeschlossen).
- Textnachrichten und Bild-Uploads.
- Realtime WebSocket-Unterstützung (`GET /api/swaps/{id}/ws`).

### 6. Bewertungen ⭐
- Nach Tauschabschluss bewerten sich beide Parteien gegenseitig (1–5 Sterne).
- Detailbewertungen für:
  - **Zuverlässigkeit** (1–5)
  - **Kommunikation** (1–5)
  - **Zustand des Objekts** (1–5)
  - **Einhaltung der Vereinbarung** (1–5)
- Das Benutzerprofil aktualisiert automatisch den Gesamtdurchschnitt und die Bewertungsanzahl.

### 7. Tausch-Matching (Smart 2-Way Match) ⭐
- Die Plattform erkennt automatisch passende Tauschpartner:
  - Max bietet *PS5* und sucht *iPhone*.
  - Lisa bietet *iPhone* und sucht *PS5*.
- Der Endpunkt `GET /api/swaps/matches` identifiziert solche Konstellationen und liefert den Match mit Score (bis zu 100/100) und Erklärung:
  `"🔥 Perfekter Tausch-Match: Du besitzt, was dein Partner sucht, und dein Partner besitzt, was du suchst!"`

### 8. Suche & "Ich suche..." (Reverse Trade Interest Search)
- Volltextsuche nach Objekten (`?q=...`), Kategoriefilter, Zustand, Preis/Wertbereich und Distanzfilter.
- **"Ich suche..."-Suche** (`?trade_interest=PS5`): Findet Objekte, deren Besitzer nach dem Suchbegriff suchen.

### 9. Benachrichtigungen 🔔 & Admin-Bereich
- Benachrichtigungen für: neue Tauschanfrage, Gegenangebot, Tausch angenommen/abgelehnt, neue Nachricht, Tausch abgeschlossen, Bewertung erhalten, Favorit.
- Admin-Dashboard (`GET /api/admin/stats`), Moderation gemeldeter Inhalte und Benutzer-Sperren (`ban`/`unban`).

---

## 🛠️ Schnellstart

### Go-Backend starten

1. **Abhängigkeiten & Build**:
   ```bash
   cd backend
   go build ./cmd/swappy-backend
   ```

2. **Backend ausführen**:
   ```bash
   ./swappy-backend
   ```
   *Standardmäßig lauscht das Backend auf `http://0.0.0.0:8080` und verwendet `swappy.db` (SQLite).*

3. **Backend-Tests ausführen**:
   ```bash
   go test -v ./internal/server
   ```

### Mit Docker Compose starten

```bash
docker compose up -d
```

---

## 📱 Swift Client & E2E-Testsuite

Im Verzeichnis `frontend/` befindet sich ein Swift 6 Package mit modernem, typisiertem API-Client (`async/await`, `URLSession`, `Codable`) und ein Testrunner, der den gesamten Benutzer- und Tausch-Workflow live gegen das Backend verifiziert.

1. **Swift Unit-Tests ausführen**:
   ```bash
   cd frontend
   swift test
   ```

2. **End-to-End Integrationstest gegen das Backend laufen lassen**:
   ```bash
   cd frontend
   SWAPPY_URL=http://localhost:8080 swift run SwappyTestRunner
   ```

---

## 📑 API-Übersicht

| Methode | Endpunkt | Auth | Beschreibung |
|---|---|---|---|
| `GET` | `/health` | Nein | Server-Status |
| `POST` | `/api/auth/register` | Nein | Benutzerregistrierung |
| `POST` | `/api/auth/login` | Nein | Login & JWT-Generierung |
| `GET` | `/api/auth/me` | Ja | Eigenes Profil abrufen |
| `GET` | `/api/users/{id}` | Nein | Öffentliches Profil mit aktiven & getauschten Objekten |
| `PUT` | `/api/users/profile` | Ja | Eigenes Profil bearbeiten |
| `POST` | `/api/users/{id}/block` | Ja | Benutzer blockieren |
| `POST` | `/api/items` | Ja | Neues Objekt zum Tausch einstellen |
| `GET` | `/api/items` | Optional | Marktplatz-Suche, Filter & `trade_interest` |
| `GET` | `/api/items/{id}` | Optional | Objektdetails & Ansichtszähler |
| `POST` | `/api/items/{id}/favorite` | Ja | Objekt favorisieren / entfavorisieren |
| `GET` | `/api/favorites` | Ja | Eigene Merkliste abrufen |
| `GET` | `/api/feed/home` | Optional | Startseite mit Kategorien, Neuesten & Beliebten |
| `POST` | `/api/swaps` | Ja | **Tauschanfrage senden (Prüft Regel 4: mind. 1 aktives Objekt)** |
| `GET` | `/api/swaps` | Ja | Eigene Tauschgeschäfte auflisten |
| `GET` | `/api/swaps/{id}` | Ja | Tauschdetails & Verlauf |
| `POST` | `/api/swaps/{id}/counter`| Ja | **Gegenangebot machen (andere Objekte / Wertausgleich)** |
| `POST` | `/api/swaps/{id}/accept` | Ja | Tauschangebot annehmen (Status -> `agreed`) |
| `POST` | `/api/swaps/{id}/reject` | Ja | Tausch ablehnen |
| `POST` | `/api/swaps/{id}/complete`| Ja | Tausch abschließen (Objekte werden `traded` 🔴) |
| `GET` | `/api/swaps/matches` | Ja | **Smart Tausch-Matching (2-Wege-Match-Erkennung)** |
| `GET` | `/api/swaps/{id}/messages`| Ja | Chatverlauf für Tausch abrufen |
| `POST` | `/api/swaps/{id}/messages`| Ja | Chatnachricht senden |
| `GET` | `/api/swaps/{id}/ws` | Ja | WebSocket-Verbindung für Live-Chat |
| `POST` | `/api/swaps/{id}/reviews`| Ja | Tauschpartner bewerten (1–5 Sterne & Kriterien) |
| `GET` | `/api/users/{id}/reviews`| Nein | Erhaltene Bewertungen eines Nutzers |
| `GET` | `/api/notifications` | Ja | Benachrichtigungen abrufen |
| `POST` | `/api/upload` | Ja | Bild-Upload (JPEG, PNG, WebP) |
| `GET` | `/api/admin/stats` | Admin | Plattformstatistiken |
| `GET` | `/api/admin/reports` | Admin | Gemeldete Vorfälle einsehen |
