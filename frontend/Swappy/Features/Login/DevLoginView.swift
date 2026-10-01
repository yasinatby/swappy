import SwiftUI

/// Placeholder login while Sign in with Apple is not built: a developer picks
/// (or creates) a test profile and is logged in without a password. Only works
/// against a backend started with SWAPPY_DEV_LOGIN=true.
struct DevLoginView: View {
    @Environment(AppSession.self) private var session
    @State private var showDevSheet = false

    var body: some View {
        VStack(spacing: 24) {
            Spacer()
            Image(systemName: "arrow.left.arrow.right.circle.fill")
                .font(.system(size: 88))
                .foregroundStyle(Color.swappyAccent)
            VStack(spacing: 8) {
                Text("Swappy")
                    .font(.largeTitle.bold())
                Text("Tauschen statt kaufen.\nOhne Geld, nur Gegenzug.")
                    .multilineTextAlignment(.center)
                    .foregroundStyle(.secondary)
            }
            Spacer()
            VStack(spacing: 12) {
                Button {} label: {
                    Label("Mit Apple anmelden", systemImage: "apple.logo")
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                .tint(.primary)
                .controlSize(.large)
                .disabled(true)
                Text("Anmeldung folgt in einer späteren Version.")
                    .font(.caption)
                    .foregroundStyle(.secondary)

                Button {
                    showDevSheet = true
                } label: {
                    Label("Ich bin Dev – ohne Login testen", systemImage: "hammer.fill")
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                .controlSize(.large)
                .accessibilityIdentifier("devLoginButton")
            }
            .padding(.horizontal, 24)
            .padding(.bottom, 32)
        }
        .sheet(isPresented: $showDevSheet) {
            DevProfilePicker()
        }
    }
}

private struct DevProfilePicker: View {
    @Environment(AppSession.self) private var session
    @Environment(\.dismiss) private var dismiss

    @State private var profiles: [Profile] = []
    @State private var newName = ""
    @State private var server = ""
    @State private var isLoading = false
    @State private var error: String?

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    if isLoading && profiles.isEmpty {
                        ProgressView()
                    }
                    ForEach(profiles) { profile in
                        Button {
                            Task { await login(profile.displayName) }
                        } label: {
                            HStack {
                                Image(systemName: "person.crop.circle.fill")
                                    .font(.title2)
                                    .foregroundStyle(Color.swappyAccent)
                                VStack(alignment: .leading) {
                                    Text(profile.displayName).foregroundStyle(.primary)
                                    Text("\(profile.activeItems) aktive Gegenstände")
                                        .font(.caption)
                                        .foregroundStyle(.secondary)
                                }
                                Spacer()
                                Image(systemName: "chevron.right").foregroundStyle(.tertiary)
                            }
                        }
                        .accessibilityIdentifier("devProfile-\(profile.displayName)")
                    }
                } header: {
                    Text("Einloggen als")
                } footer: {
                    Text("Testprofile aus der Seed-Datenbank. Kein Passwort nötig.")
                }

                Section("Neues Testprofil") {
                    TextField("Name, z. B. clara", text: $newName)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                    Button("Anlegen und einloggen") {
                        Task { await login(newName) }
                    }
                    .disabled(newName.trimmingCharacters(in: .whitespaces).count < 3)
                }

                Section {
                    TextField("http://localhost:8080", text: $server)
                        .keyboardType(.URL)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .onSubmit { Task { await applyServer() } }
                    Button("Verbinden") { Task { await applyServer() } }
                } header: {
                    Text("Backend")
                } footer: {
                    Text("Im Simulator funktioniert localhost. Auf einem echten iPhone die IP-Adresse des Macs eintragen, z. B. http://192.168.1.20:8080.")
                }
            }
            .navigationTitle("Dev-Login")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Schließen") { dismiss() }
                }
            }
            .disabled(isLoading)
            .errorAlert($error)
            .task {
                server = session.serverURL
                await loadProfiles()
            }
        }
    }

    private func loadProfiles() async {
        isLoading = true
        defer { isLoading = false }
        do {
            profiles = try await session.api.devProfiles()
        } catch let APIError.server(status, _, _) where status == 404 {
            error = "Dev-Login ist im Backend nicht aktiv. Starte es mit SWAPPY_DEV_LOGIN=true."
        } catch {
            self.error = errorMessage(for: error)
        }
    }

    private func applyServer() async {
        guard session.setServer(server) else {
            error = "Ungültige Adresse."
            return
        }
        await loadProfiles()
    }

    private func login(_ name: String) async {
        isLoading = true
        defer { isLoading = false }
        do {
            try await session.devLogin(displayName: name.trimmingCharacters(in: .whitespaces))
            dismiss()
        } catch {
            self.error = errorMessage(for: error)
        }
    }
}
