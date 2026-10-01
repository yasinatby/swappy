import SwiftUI

struct ProfileView: View {
    @Environment(AppSession.self) private var session
    let api: SwappyAPI

    @State private var showEdit = false
    @State private var confirmDelete = false
    @State private var isDeleting = false
    @State private var error: String?

    init(api: SwappyAPI) {
        self.api = api
    }

    var body: some View {
        List {
            if let me = session.currentUser {
                Section {
                    HStack(spacing: 16) {
                        Image(systemName: "person.crop.circle.fill")
                            .font(.system(size: 56))
                            .foregroundStyle(Color.swappyAccent)
                        VStack(alignment: .leading, spacing: 4) {
                            Text(me.displayName).font(.title2.bold())
                            if let postalCode = me.postalCode, !postalCode.isEmpty {
                                Label("PLZ \(postalCode)", systemImage: "mappin.and.ellipse")
                                    .font(.subheadline)
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }
                    .padding(.vertical, 4)
                    HStack {
                        stat("\(me.activeItems)", "aktive Gegenstände")
                        Divider()
                        stat("\(me.completedTrades)", "Tausche")
                    }
                }
                Section {
                    Button("Profil bearbeiten", systemImage: "pencil") { showEdit = true }
                    NavigationLink(value: Route.activity) {
                        Label("Aktivität", systemImage: "bell")
                            .badge(session.unreadNotifications)
                    }
                    NavigationLink(value: Route.blockedUsers) {
                        Label("Blockierte Nutzer", systemImage: "hand.raised")
                    }
                }
            }

            Section {
                LabeledContent("Backend", value: session.serverURL)
                Button("Abmelden", systemImage: "rectangle.portrait.and.arrow.right") {
                    session.logout()
                }
            } header: {
                Text("Entwicklermodus")
            } footer: {
                Text("Angemeldet über den Dev-Login. Sign in with Apple folgt.")
            }

            Section {
                Button(role: .destructive) {
                    confirmDelete = true
                } label: {
                    if isDeleting {
                        ProgressView()
                    } else {
                        Label("Konto löschen", systemImage: "trash")
                    }
                }
                .disabled(isDeleting)
            } footer: {
                Text("Deine Gegenstände werden entfernt und offene Angebote storniert. Abgeschlossene Tausche bleiben anonymisiert erhalten.")
            }
        }
        .navigationTitle("Profil")
        .sheet(isPresented: $showEdit) {
            EditProfileView(api: api)
        }
        .confirmationDialog("Konto endgültig löschen?", isPresented: $confirmDelete, titleVisibility: .visible) {
            Button("Konto löschen", role: .destructive) { Task { await deleteAccount() } }
        } message: {
            Text("Das kann nicht rückgängig gemacht werden.")
        }
        .task { await refresh() }
        .refreshable { await refresh() }
        .errorAlert($error)
    }

    private func stat(_ value: String, _ label: String) -> some View {
        VStack {
            Text(value).font(.title3.bold())
            Text(label).font(.caption).foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity)
    }

    private func refresh() async {
        if let me = try? await api.me() {
            session.updateCurrentUser(me)
        }
        await session.refreshBadges()
    }

    private func deleteAccount() async {
        isDeleting = true
        defer { isDeleting = false }
        do {
            try await api.deleteAccount()
            session.logout()
        } catch {
            self.error = errorMessage(for: error)
        }
    }
}

struct EditProfileView: View {
    @Environment(AppSession.self) private var session
    @Environment(\.dismiss) private var dismiss
    let api: SwappyAPI

    @State private var displayName = ""
    @State private var postalCode = ""
    @State private var isSaving = false
    @State private var error: String?

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    TextField("Anzeigename", text: $displayName)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                } header: {
                    Text("Anzeigename")
                } footer: {
                    Text("3–30 Zeichen: Buchstaben, Ziffern, _ . -")
                }
                Section {
                    TextField("Postleitzahl", text: $postalCode)
                        .keyboardType(.numberPad)
                } header: {
                    Text("Postleitzahl")
                } footer: {
                    Text("Andere sehen nur eine gerundete Entfernung („ca. 12 km“), nie deine Adresse.")
                }
            }
            .navigationTitle("Profil bearbeiten")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Abbrechen") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Sichern") { Task { await save() } }
                        .disabled(isSaving)
                }
            }
            .errorAlert($error)
            .onAppear {
                displayName = session.currentUser?.displayName ?? ""
                postalCode = session.currentUser?.postalCode ?? ""
            }
        }
    }

    private func save() async {
        isSaving = true
        defer { isSaving = false }
        do {
            let profile = try await api.updateMe(displayName: displayName, postalCode: postalCode)
            session.updateCurrentUser(profile)
            dismiss()
        } catch {
            self.error = errorMessage(for: error)
        }
    }
}

struct PublicProfileView: View {
    @Environment(\.dismiss) private var dismiss
    let api: SwappyAPI
    let profileId: String

    @State private var profile: Profile?
    @State private var items: [Item] = []
    @State private var notFound = false
    @State private var showReport = false
    @State private var confirmBlock = false
    @State private var error: String?

    private let columns = [GridItem(.flexible(), spacing: 12), GridItem(.flexible(), spacing: 12)]

    var body: some View {
        Group {
            if let profile {
                ScrollView {
                    VStack(spacing: 8) {
                        Image(systemName: "person.crop.circle.fill")
                            .font(.system(size: 72))
                            .foregroundStyle(.secondary)
                        Text(profile.displayName).font(.title2.bold())
                        HStack(spacing: 12) {
                            if let distance = distanceText(profile.distanceKm) {
                                Label(distance, systemImage: "location")
                            }
                            Label("\(profile.completedTrades) Tausche", systemImage: "arrow.left.arrow.right")
                            Label("seit \(profile.createdAt.monthYearGerman)", systemImage: "calendar")
                        }
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    }
                    .padding()

                    LazyVGrid(columns: columns, spacing: 16) {
                        ForEach(items) { item in
                            NavigationLink(value: Route.item(item.id)) {
                                ItemCard(item: item, imageURL: api.imageURL(item.coverPath))
                            }
                            .buttonStyle(.plain)
                        }
                    }
                    .padding(.horizontal)
                    if items.isEmpty {
                        Text("Gerade nichts zum Tauschen eingestellt.")
                            .foregroundStyle(.secondary)
                            .padding()
                    }
                }
                .toolbar {
                    ToolbarItem(placement: .topBarTrailing) {
                        Menu {
                            Button("Nutzer melden", systemImage: "flag") { showReport = true }
                            Button("Blockieren", systemImage: "hand.raised", role: .destructive) { confirmBlock = true }
                        } label: {
                            Image(systemName: "ellipsis.circle")
                        }
                    }
                }
                .confirmationDialog("\(profile.displayName) blockieren?", isPresented: $confirmBlock, titleVisibility: .visible) {
                    Button("Blockieren", role: .destructive) { Task { await block() } }
                } message: {
                    Text("Ihr seht gegenseitig keine Gegenstände mehr und offene Angebote zwischen euch werden storniert.")
                }
            } else if notFound {
                ContentUnavailableView("Profil nicht verfügbar", systemImage: "person.slash")
            } else {
                ProgressView()
            }
        }
        .navigationBarTitleDisplayMode(.inline)
        .sheet(isPresented: $showReport) {
            ReportSheet(api: api, subject: .profile, subjectId: profileId)
        }
        .task { await load() }
        .refreshable { await load() }
        .errorAlert($error)
    }

    private func load() async {
        do {
            async let p = api.user(profileId)
            async let i = api.feed(ownerId: profileId)
            profile = try await p
            items = try await i
        } catch APIError.server(404, _, _) {
            notFound = true
        } catch {
            self.error = errorMessage(for: error)
        }
    }

    private func block() async {
        do {
            try await api.block(profileId)
            dismiss()
        } catch {
            self.error = errorMessage(for: error)
        }
    }
}

struct BlockedUsersView: View {
    let api: SwappyAPI
    @State private var blocked: [BlockedUser] = []
    @State private var hasLoaded = false
    @State private var error: String?

    var body: some View {
        List {
            ForEach(blocked) { user in
                HStack {
                    VStack(alignment: .leading) {
                        Text(user.displayName)
                        Text("seit \(user.blockedAt.shortGerman)")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                    Spacer()
                    Button("Aufheben") { Task { await unblock(user) } }
                        .buttonStyle(.bordered)
                }
            }
        }
        .overlay {
            if hasLoaded && blocked.isEmpty {
                ContentUnavailableView("Niemand blockiert", systemImage: "hand.raised")
            }
        }
        .navigationTitle("Blockierte Nutzer")
        .task { await load() }
        .errorAlert($error)
    }

    private func load() async {
        do {
            blocked = try await api.blockedUsers()
            hasLoaded = true
        } catch {
            self.error = errorMessage(for: error)
        }
    }

    private func unblock(_ user: BlockedUser) async {
        do {
            try await api.unblock(user.id)
            blocked.removeAll { $0.id == user.id }
        } catch {
            self.error = errorMessage(for: error)
        }
    }
}
