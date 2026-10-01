# Swappy iOS-App

SwiftUI, iOS 17+, ohne Abhängigkeiten. Das Xcode-Projekt wird aus `project.yml`
erzeugt (`brew install xcodegen`, dann `xcodegen generate`) und nicht eingecheckt.

- `Swappy/Networking`: `APIClient` (HTTP/JSON), `SwappyAPI` (alle Endpunkte), Modelle
- `Swappy/Features`: ein Ordner pro Bereich; jede View hat ein `@Observable`-Model,
  Views rufen nie direkt die API auf
- `Swappy/Shared`: Komponenten, Keychain, Bildkompression (≤ 1600 px, JPEG 0.7)

Start und Tests: siehe README im Hauptordner.
