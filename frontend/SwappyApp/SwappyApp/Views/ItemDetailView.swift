import SwiftUI

struct ItemDetailView: View {
    @EnvironmentObject var api: APIClient
    @Environment(\.dismiss) var dismiss
    let item: Item

    @State private var showProposeSheet = false
    @State private var myItems: [Item] = []
    @State private var selectedMyItemIds: Set<String> = []
    @State private var cashAdjustment: Double = 0.0
    @State private var proposalMessage: String = ""
    @State private var isSubmitting = false
    @State private var alertMessage: String? = nil
    @State private var showAlert = false

    var isOwnItem: Bool {
        guard let current = api.currentUser else { return false }
        return current.id == item.userId
    }

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 18) {
                    // Header Image Placeholder
                    ZStack(alignment: .topTrailing) {
                        RoundedRectangle(cornerRadius: 16)
                            .fill(Color.blue.opacity(0.12))
                            .frame(height: 220)
                            .overlay(
                                Image(systemName: "photo.on.rectangle.angled")
                                    .font(.system(size: 60))
                                    .foregroundColor(.blue.opacity(0.6))
                            )

                        Text(item.status == "available" ? "🟢 Verfügbar" : "🟡 In Verhandlung")
                            .font(.caption.bold())
                            .padding(.horizontal, 10)
                            .padding(.vertical, 6)
                            .background(.ultraThinMaterial)
                            .clipShape(Capsule())
                            .padding()
                    }

                    // Title & Value
                    VStack(alignment: .leading, spacing: 6) {
                        Text(item.title)
                            .font(.title2.bold())

                        HStack {
                            Text("Geschätzter Wert:")
                                .foregroundColor(.secondary)
                            Text("\(Int(item.estimatedValue)) €")
                                .font(.headline)
                                .foregroundColor(.blue)

                            Spacer()

                            Text(item.condition)
                                .font(.caption.bold())
                                .padding(.horizontal, 8)
                                .padding(.vertical, 4)
                                .background(Color.blue.opacity(0.1))
                                .clipShape(Capsule())
                        }
                    }

                    Divider()

                    // Trade Interests ("Was gesucht wird")
                    VStack(alignment: .leading, spacing: 8) {
                        Label("Tauschinteresse / Gesucht wird:", systemImage: "arrow.triangle.swap")
                            .font(.headline)
                            .foregroundColor(.primary)

                        if item.tradeInterests.isEmpty {
                            Text("Offen für Vorschläge")
                                .font(.subheadline)
                                .foregroundColor(.secondary)
                        } else {
                            FlowLayout(spacing: 8) {
                                ForEach(item.tradeInterests, id: \.self) { interest in
                                    Text(interest)
                                        .font(.subheadline)
                                        .padding(.horizontal, 12)
                                        .padding(.vertical, 6)
                                        .background(Color.green.opacity(0.15))
                                        .foregroundColor(.green.opacity(0.9))
                                        .clipShape(Capsule())
                                }
                            }
                        }
                    }

                    Divider()

                    // Description
                    VStack(alignment: .leading, spacing: 8) {
                        Text("Beschreibung")
                            .font(.headline)
                        Text(item.description.isEmpty ? "Keine nähere Beschreibung angegeben." : item.description)
                            .font(.body)
                            .foregroundColor(.secondary)
                    }

                    Divider()

                    // Owner Info
                    if let owner = item.owner {
                        VStack(alignment: .leading, spacing: 8) {
                            Text("Anbieter")
                                .font(.headline)

                            HStack(spacing: 12) {
                                Circle()
                                    .fill(Color.blue.opacity(0.2))
                                    .frame(width: 44, height: 44)
                                    .overlay(
                                        Text(owner.username.prefix(1).uppercased())
                                            .font(.headline.bold())
                                            .foregroundColor(.blue)
                                    )

                                VStack(alignment: .leading, spacing: 2) {
                                    Text(owner.username)
                                        .font(.subheadline.bold())
                                    HStack(spacing: 8) {
                                        Text("⭐ \(String(format: "%.1f", owner.ratingAverage)) (\(owner.ratingCount))")
                                            .font(.caption)
                                            .foregroundColor(.orange)
                                        Text("• \(owner.completedTradesCount) Täusche")
                                            .font(.caption)
                                            .foregroundColor(.secondary)
                                    }
                                }
                                Spacer()
                            }
                            .padding()
                            .background(Color(.secondarySystemGroupedBackground))
                            .clipShape(RoundedRectangle(cornerRadius: 12))
                        }
                    }
                }
                .padding()
            }
            .background(Color(.systemGroupedBackground))
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Schließen") { dismiss() }
                }
            }
            .safeAreaInset(edge: .bottom) {
                if !isOwnItem {
                    Button {
                        Task { await openProposeSheet() }
                    } label: {
                        HStack {
                            Image(systemName: "arrow.triangle.swap")
                            Text("Tausch vorschlagen")
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
                }
            }
            .sheet(isPresented: $showProposeSheet) {
                proposeSwapSheet
            }
            .alert("Hinweis", isPresented: $showAlert) {
                Button("OK") {}
            } message: {
                Text(alertMessage ?? "")
            }
        }
    }

    private var proposeSwapSheet: some View {
        NavigationStack {
            Form {
                Section("Du möchtest:") {
                    HStack {
                        Text(item.title)
                            .bold()
                        Spacer()
                        Text("\(Int(item.estimatedValue)) €")
                            .foregroundColor(.blue)
                    }
                }

                Section("Wähle deine Tauschobjekte:") {
                    if myItems.isEmpty {
                        VStack(alignment: .leading, spacing: 6) {
                            Text("⚠️ Keine aktiven Objekte vorhanden")
                                .font(.subheadline.bold())
                                .foregroundColor(.red)
                            Text("Regel: Du musst mindestens ein eigenes Objekt veröffentlicht haben, um einen Tausch vorzuschlagen.")
                                .font(.caption)
                                .foregroundColor(.secondary)
                        }
                    } else {
                        ForEach(myItems) { myItem in
                            HStack {
                                VStack(alignment: .leading) {
                                    Text(myItem.title).bold()
                                    Text("Wert: \(Int(myItem.estimatedValue)) €")
                                        .font(.caption)
                                        .foregroundColor(.secondary)
                                }
                                Spacer()
                                if selectedMyItemIds.contains(myItem.id) {
                                    Image(systemName: "checkmark.circle.fill")
                                        .foregroundColor(.blue)
                                } else {
                                    Image(systemName: "circle")
                                        .foregroundColor(.secondary)
                                }
                            }
                            .contentShape(Rectangle())
                            .onTapGesture {
                                if selectedMyItemIds.contains(myItem.id) {
                                    selectedMyItemIds.remove(myItem.id)
                                } else {
                                    selectedMyItemIds.insert(myItem.id)
                                }
                            }
                        }
                    }
                }

                Section("Optionaler Wertausgleich (€):") {
                    HStack {
                        Text("Zuzahlung:")
                        Spacer()
                        TextField("0", value: $cashAdjustment, format: .number)
                            .keyboardType(.numbersAndPunctuation)
                            .multilineTextAlignment(.trailing)
                        Text("€")
                    }
                }

                Section("Nachricht an Anbieter:") {
                    TextField("Ich würde gerne mein ... gegen dein ... tauschen!", text: $proposalMessage, axis: .vertical)
                        .lineLimit(3...6)
                }
            }
            .navigationTitle("Tausch vorschlagen")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Abbrechen") { showProposeSheet = false }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Angebot senden") {
                        Task { await submitProposal() }
                    }
                    .disabled(selectedMyItemIds.isEmpty || isSubmitting)
                }
            }
        }
    }

    private func openProposeSheet() async {
        guard let current = api.currentUser else {
            alertMessage = "Bitte melde dich zuerst im Profil-Tab an!"
            showAlert = true
            return
        }

        do {
            let items = try await api.fetchItems()
            self.myItems = items.filter { $0.userId == current.id && $0.status == "available" }
            if self.myItems.isEmpty {
                alertMessage = "Tauschregel: Du musst erst mindestens ein eigenes Objekt veröffentlichen, bevor du einen Tausch vorschlagen kannst!"
                showAlert = true
                return
            }
            self.showProposeSheet = true
        } catch {
            alertMessage = error.localizedDescription
            showAlert = true
        }
    }

    private func submitProposal() async {
        isSubmitting = true
        defer { isSubmitting = false }
        do {
            _ = try await api.proposeSwap(
                targetItemId: item.id,
                offeredItemIds: Array(selectedMyItemIds),
                cashAdjustment: cashAdjustment,
                message: proposalMessage.isEmpty ? "Ich interessiere mich für deinen Tauschgegenstand!" : proposalMessage
            )
            showProposeSheet = false
            alertMessage = "🎉 Tauschangebot erfolgreich versendet!"
            showAlert = true
        } catch {
            alertMessage = error.localizedDescription
            showAlert = true
        }
    }
}

// Simple FlowLayout helper for tags
struct FlowLayout: Layout {
    var spacing: CGFloat = 8

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let width = proposal.width ?? 300
        var height: CGFloat = 0
        var x: CGFloat = 0
        var rowHeight: CGFloat = 0

        for subview in subviews {
            let size = subview.sizeThatFits(.unspecified)
            if x + size.width > width {
                x = 0
                height += rowHeight + spacing
                rowHeight = 0
            }
            rowHeight = max(rowHeight, size.height)
            x += size.width + spacing
        }
        height += rowHeight
        return CGSize(width: width, height: height)
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        var x = bounds.minX
        var y = bounds.minY
        var rowHeight: CGFloat = 0

        for subview in subviews {
            let size = subview.sizeThatFits(.unspecified)
            if x + size.width > bounds.maxX {
                x = bounds.minX
                y += rowHeight + spacing
                rowHeight = 0
            }
            subview.place(at: CGPoint(x: x, y: y), proposal: ProposedViewSize(size))
            rowHeight = max(rowHeight, size.height)
            x += size.width + spacing
        }
    }
}
