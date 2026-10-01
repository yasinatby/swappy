# Swappy Frontend & Swift Client

Dieses Verzeichnis enthält die Swift-Implementierung für den Zugriff auf das Swappy-Backend, kompatibel mit iOS 16+ und macOS 13+.

## 📦 Komponenten

1. **`SwappyClient` (Library)**:
   - `SwappyAPIClient`: `actor` mit modernen `async/await` Methoden und robuster Fehlerbehandlung.
   - `Models.swift`: Typisierte, `Codable` und `Sendable` Datenmodelle für Benutzer, Objekte, Tauschprozesse, Chatnachrichten, Bewertungen, Benachrichtigungen und Smart-Matches.

2. **`SwappyTestRunner` (Executable)**:
   - Führt ein automatisches End-to-End-Testszenario live gegen das Backend aus:
     - Benutzerregistrierung & Authentifizierung
     - Verifikation von **Regel 4**: Tauschanfrage ohne aktives Objekt wird abgewiesen
     - Objekterstellung mit Tauschinteressen
     - **Tausch-Matching ⭐**: Erkennung gegenseitiger Interessen (PS5 ↔ iPhone 15)
     - Rückwärtssuche nach Tauschinteressen
     - Tauschvorschlag mit Wertausgleich
     - Gegenangebot (Counter-Offer)
     - Annahme & Objekt-Statusübergang (`available` -> `requested`)
     - Chat-Nachrichtenaustausch
     - Tauschabschluss & Objekt-Statusübergang (`requested` -> `traded` 🔴)
     - Detaillierte Sterne-Bewertung
     - Merkliste / Favoriten & Benachrichtigungen
     - Admin-Statistiken

3. **`SwappyClientTests` (Unit Tests)**:
   - Swift-Testsuite (`swift test`) zur Validierung der Modell-Serialisierung und Decodierung.

## 🚀 Ausführung

```bash
# Unit-Tests
swift test

# E2E-Runner gegen lokales Backend
SWAPPY_URL=http://localhost:8080 swift run SwappyTestRunner
```
