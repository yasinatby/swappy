import SwiftUI

struct SwapDetailView: View {
    @EnvironmentObject var api: APIClient
    @Environment(\.dismiss) var dismiss
    let swap: Swap
    var onUpdated: () -> Void

    @State private var currentSwap: Swap
    @State private var showCounterSheet = false
    @State private var showReviewSheet = false
    @State private var counterCashAdjustment: Double = 0.0
    @State private var counterMessage: String = ""
    @State private var reviewRating: Int = 5
    @State private var reviewComment: String = ""
    @State private var isLoading = false
    @State private var alertMessage: String? = nil
    @State private var showAlert = false

    init(swap: Swap, onUpdated: @escaping () -> Void) {
        self.swap = swap
        self.onUpdated = onUpdated
        _currentSwap = State(initialValue: swap)
    }

    var isInitiator: Bool {
        api.currentUser?.id == currentSwap.initiatorId
    }

    var partner: UserPublicProfile? {
        isInitiator ? currentSwap.recipient : currentSwap.initiator
    }

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(spacing: 20) {
                    // Status Card
                    statusCard

                    // Target & Offered Comparison
                    itemsComparisonCard

                    // Cash Adjustment
                    if currentSwap.cashAdjustment != 0 {
                        cashAdjustmentCard
                    }

                    // Handover & Negotiation History
                    historySection

                    // Chat Preview / Navigation
                    NavigationLink {
                        ChatView(swapId: currentSwap.id)
                    } label: {
                        HStack {
                            Image(systemName: "bubble.left.and.bubble.right.fill")
                                .foregroundColor(.blue)
                            Text("Tausch-Chat öffnen")
                                .bold()
                            Spacer()
                            Image(systemName: "chevron.right")
                                .foregroundColor(.secondary)
                        }
                        .padding()
                        .background(Color(.secondarySystemGroupedBackground))
                        .clipShape(RoundedRectangle(cornerRadius: 14))
                    }
                    .padding(.horizontal)
                }
                .padding(.vertical)
            }
            .background(Color(.systemGroupedBackground))
            .navigationTitle("Tausch-Details")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Schließen") { dismiss() }
                }
            }
            .safeAreaInset(edge: .bottom) {
                actionButtons
            }
            .sheet(isPresented: $showCounterSheet) {
                counterOfferSheet
            }
            .sheet(isPresented: $showReviewSheet) {
                reviewSheet
            }
            .alert("Tausch", isPresented: $showAlert) {
                Button("OK") {}
            } message: {
                Text(alertMessage ?? "")
            }
        }
    }

    private var statusCard: some View {
        VStack(spacing: 8) {
            Text(statusTitle(currentSwap.status))
                .font(.headline.bold())
                .foregroundColor(statusColor(currentSwap.status))

            Text(statusSubtitle(currentSwap.status))
                .font(.subheadline)
                .foregroundColor(.secondary)
                .multilineTextAlignment(.center)
        }
        .frame(maxWidth: .infinity)
        .padding()
        .background(Color(.secondarySystemGroupedBackground))
        .clipShape(RoundedRectangle(cornerRadius: 16))
        .padding(.horizontal)
    }

    private var itemsComparisonCard: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text("Übersicht")
                .font(.headline)

            HStack(spacing: 12) {
                VStack(alignment: .leading, spacing: 4) {
                    Text("ZIELOBJEKT:")
                        .font(.caption2.bold())
                        .foregroundColor(.secondary)
                    Text(currentSwap.targetItem?.title ?? "Zielobjekt")
                        .font(.subheadline.bold())
                    Text("Wert: \(Int(currentSwap.targetItem?.estimatedValue ?? 0)) €")
                        .font(.caption)
                        .foregroundColor(.blue)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(10)
                .background(Color(.tertiarySystemGroupedBackground))
                .clipShape(RoundedRectangle(cornerRadius: 12))

                Image(systemName: "arrow.triangle.swap")
                    .foregroundColor(.secondary)

                VStack(alignment: .leading, spacing: 4) {
                    Text("ANGEBOTEN:")
                        .font(.caption2.bold())
                        .foregroundColor(.secondary)
                    Text(currentSwap.offeredItems?.first?.title ?? "Angebotenes Objekt")
                        .font(.subheadline.bold())
                    Text("Wert: \(Int(currentSwap.offeredItems?.first?.estimatedValue ?? 0)) €")
                        .font(.caption)
                        .foregroundColor(.green)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(10)
                .background(Color(.tertiarySystemGroupedBackground))
                .clipShape(RoundedRectangle(cornerRadius: 12))
            }
        }
        .padding()
        .background(Color(.secondarySystemGroupedBackground))
        .clipShape(RoundedRectangle(cornerRadius: 16))
        .padding(.horizontal)
    }

    private var cashAdjustmentCard: some View {
        HStack {
            Image(systemName: "eurosign.circle.fill")
                .foregroundColor(.blue)
                .font(.title3)
            VStack(alignment: .leading, spacing: 2) {
                Text("Wertausgleich")
                    .font(.subheadline.bold())
                Text(currentSwap.cashAdjustment > 0 ? "Initiator zahlt +\(Int(currentSwap.cashAdjustment)) € dazu" : "Empfänger zahlt \(Int(-currentSwap.cashAdjustment)) € dazu")
                    .font(.caption)
                    .foregroundColor(.secondary)
            }
            Spacer()
            Text("\(Int(currentSwap.cashAdjustment)) €")
                .font(.headline)
                .foregroundColor(.blue)
        }
        .padding()
        .background(Color(.secondarySystemGroupedBackground))
        .clipShape(RoundedRectangle(cornerRadius: 14))
        .padding(.horizontal)
    }

    private var historySection: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Verlauf & Runden")
                .font(.headline)
                .padding(.horizontal)

            if let histories = currentSwap.history, !histories.isEmpty {
                VStack(spacing: 8) {
                    ForEach(histories) { h in
                        HStack(alignment: .top, spacing: 10) {
                            Circle()
                                .fill(Color.blue)
                                .frame(width: 8, height: 8)
                                .padding(.top, 6)

                            VStack(alignment: .leading, spacing: 2) {
                                Text("@\(h.actorUsername ?? "User"): \(h.action.capitalized)")
                                    .font(.caption.bold())
                                if let msg = h.message, !msg.isEmpty {
                                    Text("„\(msg)“")
                                        .font(.caption)
                                        .foregroundColor(.secondary)
                                }
                            }
                            Spacer()
                        }
                    }
                }
                .padding()
                .background(Color(.secondarySystemGroupedBackground))
                .clipShape(RoundedRectangle(cornerRadius: 14))
                .padding(.horizontal)
            }
        }
    }

    @ViewBuilder
    private var actionButtons: some View {
        VStack(spacing: 8) {
            if currentSwap.status == "pending" || currentSwap.status == "countered" {
                HStack(spacing: 12) {
                    Button {
                        Task { await accept() }
                    } label: {
                        Text("Annehmen")
                            .bold()
                            .frame(maxWidth: .infinity)
                            .padding()
                            .background(Color.green)
                            .foregroundColor(.white)
                            .clipShape(RoundedRectangle(cornerRadius: 12))
                    }

                    Button {
                        counterCashAdjustment = currentSwap.cashAdjustment
                        showCounterSheet = true
                    } label: {
                        Text("Gegenangebot")
                            .bold()
                            .frame(maxWidth: .infinity)
                            .padding()
                            .background(Color.purple)
                            .foregroundColor(.white)
                            .clipShape(RoundedRectangle(cornerRadius: 12))
                    }

                    Button {
                        Task { await reject() }
                    } label: {
                        Text("Ablehnen")
                            .bold()
                            .frame(maxWidth: .infinity)
                            .padding()
                            .background(Color.red.opacity(0.15))
                            .foregroundColor(.red)
                            .clipShape(RoundedRectangle(cornerRadius: 12))
                    }
                }
                .padding()
                .background(.ultraThinMaterial)
            } else if currentSwap.status == "agreed" {
                Button {
                    Task { await complete() }
                } label: {
                    HStack {
                        Image(systemName: "checkmark.seal.fill")
                        Text("Tausch abschließen / Bestätigen")
                            .bold()
                    }
                    .frame(maxWidth: .infinity)
                    .padding()
                    .background(Color.blue)
                    .foregroundColor(.white)
                    .clipShape(RoundedRectangle(cornerRadius: 14))
                }
                .padding()
                .background(.ultraThinMaterial)
            } else if currentSwap.status == "completed" {
                Button {
                    showReviewSheet = true
                } label: {
                    HStack {
                        Image(systemName: "star.fill")
                        Text("Tauschpartner bewerten ⭐")
                            .bold()
                    }
                    .frame(maxWidth: .infinity)
                    .padding()
                    .background(Color.orange)
                    .foregroundColor(.white)
                    .clipShape(RoundedRectangle(cornerRadius: 14))
                }
                .padding()
                .background(.ultraThinMaterial)
            }
        }
    }

    private var counterOfferSheet: some View {
        NavigationStack {
            Form {
                Section("Neuer Wertausgleich (€)") {
                    TextField("0", value: $counterCashAdjustment, format: .number)
                        .keyboardType(.numbersAndPunctuation)
                }

                Section("Nachricht zum Gegenangebot") {
                    TextField("Ich würde tauschen, wenn du noch ... dazugibst.", text: $counterMessage, axis: .vertical)
                        .lineLimit(3...5)
                }
            }
            .navigationTitle("Gegenangebot erstellen")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Abbrechen") { showCounterSheet = false }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Senden") {
                        Task { await sendCounterOffer() }
                    }
                }
            }
        }
    }

    private var reviewSheet: some View {
        NavigationStack {
            Form {
                Section("Gesamtbewertung") {
                    Picker("Sterne", selection: $reviewRating) {
                        Text("⭐⭐⭐⭐⭐ (5)").tag(5)
                        Text("⭐⭐⭐⭐ (4)").tag(4)
                        Text("⭐⭐⭐ (3)").tag(3)
                        Text("⭐⭐ (2)").tag(2)
                        Text("⭐ (1)").tag(1)
                    }
                }

                Section("Kommentar") {
                    TextField("Sehr zuverlässig, alles wie beschrieben!", text: $reviewComment, axis: .vertical)
                        .lineLimit(3...5)
                }
            }
            .navigationTitle("Partner bewerten")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Abbrechen") { showReviewSheet = false }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Absenden") {
                        Task { await submitReview() }
                    }
                }
            }
        }
    }

    private func accept() async {
        do {
            currentSwap = try await api.acceptSwap(swapId: currentSwap.id)
            onUpdated()
            alertMessage = "Tausch angenommen! Status ist nun 🔄 Tausch vereinbart."
            showAlert = true
        } catch {
            alertMessage = error.localizedDescription
            showAlert = true
        }
    }

    private func reject() async {
        do {
            currentSwap = try await api.rejectSwap(swapId: currentSwap.id)
            onUpdated()
            alertMessage = "Tausch abgelehnt."
            showAlert = true
        } catch {
            alertMessage = error.localizedDescription
            showAlert = true
        }
    }

    private func complete() async {
        do {
            currentSwap = try await api.completeSwap(swapId: currentSwap.id)
            onUpdated()
            alertMessage = "🤝 Tausch erfolgreich abgeschlossen! Die Objekte wurden als getauscht markiert."
            showAlert = true
        } catch {
            alertMessage = error.localizedDescription
            showAlert = true
        }
    }

    private func sendCounterOffer() async {
        do {
            currentSwap = try await api.counterSwap(
                swapId: currentSwap.id,
                offeredItemIds: currentSwap.offeredItems?.map(\.id) ?? [],
                cashAdjustment: counterCashAdjustment,
                message: counterMessage.isEmpty ? "Gegenangebot gesendet." : counterMessage
            )
            showCounterSheet = false
            onUpdated()
            alertMessage = "Gegenangebot versendet!"
            showAlert = true
        } catch {
            alertMessage = error.localizedDescription
            showAlert = true
        }
    }

    private func submitReview() async {
        do {
            _ = try await api.submitReview(
                swapId: currentSwap.id,
                rating: reviewRating,
                comment: reviewComment.isEmpty ? "Alles top gelaufen!" : reviewComment
            )
            showReviewSheet = false
            alertMessage = "Vielen Dank für deine Bewertung! ⭐"
            showAlert = true
        } catch {
            alertMessage = error.localizedDescription
            showAlert = true
        }
    }

    private func statusTitle(_ s: String) -> String {
        switch s {
        case "pending": return "🟡 Tauschangebot offen"
        case "countered": return "⚖️ Gegenangebot eingereicht"
        case "agreed": return "🔄 Tausch vereinbart"
        case "completed": return "🤝 Tausch abgeschlossen"
        case "rejected": return "❌ Tausch abgelehnt"
        default: return s.capitalized
        }
    }

    private func statusSubtitle(_ s: String) -> String {
        switch s {
        case "pending": return "Wartet auf Rückmeldung des Tauschpartners."
        case "countered": return "Ein Gegenangebot wurde gemacht. Bitte prüfen."
        case "agreed": return "Stimmt nun die persönliche Übergabe oder den Versand im Chat ab."
        case "completed": return "Beide Parteien haben den Tausch erfolgreich beendet."
        default: return ""
        }
    }

    private func statusColor(_ s: String) -> Color {
        switch s {
        case "pending": return .orange
        case "countered": return .purple
        case "agreed": return .blue
        case "completed": return .green
        case "rejected": return .red
        default: return .primary
        }
    }
}
