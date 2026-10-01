import SwiftUI

/// Item photo from the backend, or a category placeholder.
struct ItemThumbnail: View {
    let url: URL?
    var category: String? = nil
    var cornerRadius: CGFloat = 12

    var body: some View {
        Color(.secondarySystemBackground)
            .overlay {
                if let url {
                    AsyncImage(url: url) { phase in
                        switch phase {
                        case let .success(image): image.resizable().scaledToFill()
                        case .failure: placeholder
                        default: ProgressView()
                        }
                    }
                } else {
                    placeholder
                }
            }
            .clipShape(RoundedRectangle(cornerRadius: cornerRadius, style: .continuous))
    }

    private var placeholder: some View {
        Image(systemName: category.map(Category.symbol) ?? "photo")
            .font(.title)
            .foregroundStyle(.tertiary)
    }
}

struct StatusBadge: View {
    let text: String
    let color: Color

    var body: some View {
        Text(text)
            .font(.caption.weight(.semibold))
            .padding(.horizontal, 8)
            .padding(.vertical, 3)
            .background(color.opacity(0.15), in: Capsule())
            .foregroundStyle(color)
    }
}

extension ItemStatus {
    var color: Color {
        switch self {
        case .draft: .gray
        case .active: .green
        case .reserved: .orange
        case .traded: .blue
        case .removed, .deleted: .red
        }
    }
}

extension OfferStatus {
    var color: Color {
        switch self {
        case .pending: .orange
        case .accepted: .green
        case .completed: .blue
        case .declined, .cancelled, .expired: .gray
        }
    }
}

/// "ca. 12 km" — the backend already rounds; the app never sees coordinates.
func distanceText(_ km: Int?) -> String? {
    km.map { "ca. \($0) km" }
}

struct ItemCard: View {
    let item: Item
    let imageURL: URL?

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            ItemThumbnail(url: imageURL, category: item.category)
                .aspectRatio(1, contentMode: .fit)
            Text(item.title)
                .font(.subheadline.weight(.semibold))
                .lineLimit(2, reservesSpace: true)
                .foregroundStyle(.primary)
            HStack(spacing: 4) {
                Text(item.ownerName)
                if let distance = distanceText(item.distanceKm) {
                    Text("·")
                    Text(distance)
                }
            }
            .font(.caption)
            .foregroundStyle(.secondary)
            .lineLimit(1)
            if let wanted = item.wantedText {
                Label(wanted, systemImage: "arrow.left.arrow.right")
                    .font(.caption)
                    .foregroundStyle(Color.swappyAccent)
                    .lineLimit(1)
            }
        }
        .contentShape(Rectangle())
    }
}

struct ItemRow: View {
    let item: Item
    let imageURL: URL?

    var body: some View {
        HStack(spacing: 12) {
            ItemThumbnail(url: imageURL, category: item.category, cornerRadius: 8)
                .frame(width: 56, height: 56)
            VStack(alignment: .leading, spacing: 4) {
                Text(item.title).font(.body.weight(.medium)).lineLimit(1)
                Text("\(Category.label(for: item.category)) · \(item.condition.label)")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer()
            StatusBadge(text: item.status.label, color: item.status.color)
        }
    }
}

// MARK: - Errors

extension View {
    /// Shows `error` as an alert and clears it on dismiss.
    func errorAlert(_ error: Binding<String?>) -> some View {
        alert("Hoppla", isPresented: Binding(
            get: { error.wrappedValue != nil },
            set: { if !$0 { error.wrappedValue = nil } }
        )) {
            Button("OK", role: .cancel) {}
        } message: {
            Text(error.wrappedValue ?? "")
        }
    }
}

func errorMessage(for error: Error) -> String? {
    if error is CancellationError { return nil }
    if case .unauthorized = error as? APIError { return nil } // session handles it
    return (error as? LocalizedError)?.errorDescription ?? error.localizedDescription
}

// MARK: - Reporting (App Store guideline 1.2)

struct ReportSheet: View {
    let api: SwappyAPI
    let subject: ReportSubject
    let subjectId: String
    @Environment(\.dismiss) private var dismiss

    @State private var reason: ReportReason = .spam
    @State private var note = ""
    @State private var isSending = false
    @State private var error: String?

    var body: some View {
        NavigationStack {
            Form {
                Section("Grund") {
                    Picker("Grund", selection: $reason) {
                        ForEach(ReportReason.allCases) { Text($0.label).tag($0) }
                    }
                    .pickerStyle(.inline)
                    .labelsHidden()
                }
                Section {
                    TextField("Was ist passiert? (optional)", text: $note, axis: .vertical)
                        .lineLimit(3 ... 6)
                } footer: {
                    Text("Wir prüfen jede Meldung innerhalb von 24 Stunden.")
                }
            }
            .navigationTitle(subject == .item ? "Gegenstand melden" : "Nutzer melden")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Abbrechen") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Senden") { Task { await send() } }
                        .disabled(isSending)
                }
            }
            .errorAlert($error)
        }
    }

    private func send() async {
        isSending = true
        defer { isSending = false }
        do {
            let trimmed = note.trimmingCharacters(in: .whitespacesAndNewlines)
            try await api.report(subject, id: subjectId, reason: reason, note: trimmed.isEmpty ? nil : trimmed)
            dismiss()
        } catch {
            self.error = errorMessage(for: error)
        }
    }
}

// MARK: - Dates (always German, independent of the device language)

extension Date {
    private static let german = Locale(identifier: "de_DE")

    /// "in 2 Wochen", "vor 3 Minuten"
    var relativeGerman: String {
        let formatter = RelativeDateTimeFormatter()
        formatter.locale = Self.german
        formatter.unitsStyle = .full
        return formatter.localizedString(for: self, relativeTo: .now)
    }

    /// "1. Okt. 2026"
    var shortGerman: String {
        formatted(.dateTime.day().month(.abbreviated).year().locale(Self.german))
    }

    /// "Oktober 2026"
    var monthYearGerman: String {
        formatted(.dateTime.month(.wide).year().locale(Self.german))
    }
}

/// "läuft in 14 Tagen ab" — rounded up, so an offer never looks shorter-lived
/// than it is.
func expiryText(_ date: Date, now: Date = .now) -> String {
    let seconds = date.timeIntervalSince(now)
    if seconds <= 0 { return "abgelaufen" }
    let hours = Int((seconds / 3600).rounded(.up))
    if hours < 24 { return hours == 1 ? "läuft in 1 Stunde ab" : "läuft in \(hours) Stunden ab" }
    let days = Int((seconds / 86400).rounded(.up))
    return days == 1 ? "läuft morgen ab" : "läuft in \(days) Tagen ab"
}
