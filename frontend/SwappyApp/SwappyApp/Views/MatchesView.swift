import SwiftUI

struct MatchesView: View {
    @EnvironmentObject var api: APIClient
    @State private var matches: [SwapMatch] = []
    @State private var isLoading = false
    @State private var selectedMatch: SwapMatch? = nil
    @State private var alertMessage: String? = nil
    @State private var showAlert = false

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(spacing: 16) {
                    // Header explanation
                    VStack(alignment: .leading, spacing: 6) {
                        HStack {
                            Text("🔥 Smarte Tausch-Matches")
                                .font(.title2.bold())
                            Spacer()
                        }
                        Text("Die Plattform erkennt automatisch Tauschpartner: Du besitzt, was der andere sucht – und umgekehrt!")
                            .font(.subheadline)
                            .foregroundColor(.secondary)
                    }
                    .padding(.horizontal)

                    if isLoading {
                        ProgressView()
                            .padding(.top, 40)
                    } else if matches.isEmpty {
                        VStack(spacing: 12) {
                            Image(systemName: "sparkles")
                                .font(.system(size: 48))
                                .foregroundColor(.orange)
                            Text("Noch keine Matches gefunden")
                                .font(.headline)
                            Text("Stelle eigene Objekte mit konkreten Tauschinteressen ein, um automatische Matches zu erhalten.")
                                .font(.subheadline)
                                .foregroundColor(.secondary)
                                .multilineTextAlignment(.center)
                                .padding(.horizontal)
                        }
                        .padding(.top, 40)
                    } else {
                        ForEach(matches) { match in
                            MatchCard(match: match) {
                                Task { await proposeMatch(match) }
                            }
                        }
                        .padding(.horizontal)
                    }
                }
                .padding(.vertical)
            }
            .background(Color(.systemGroupedBackground))
            .navigationTitle("Tausch-Matches")
            .refreshable {
                await loadMatches()
            }
            .task {
                await loadMatches()
            }
            .alert("Tausch", isPresented: $showAlert) {
                Button("OK") {}
            } message: {
                Text(alertMessage ?? "")
            }
        }
    }

    private func loadMatches() async {
        guard api.isAuthenticated else { return }
        isLoading = true
        defer { isLoading = false }
        do {
            self.matches = try await api.fetchMatches()
        } catch {
            print("Failed to fetch matches: \(error)")
        }
    }

    private func proposeMatch(_ match: SwapMatch) async {
        do {
            _ = try await api.proposeSwap(
                targetItemId: match.theirItem.id,
                offeredItemIds: [match.myItem.id],
                cashAdjustment: 0.0,
                message: "Hallo! Wir haben ein direktes Tausch-Match: Mein(e) \(match.myItem.title) gegen dein(e) \(match.theirItem.title)!"
            )
            alertMessage = "🎉 Tauschangebot erfolgreich versendet!"
            showAlert = true
        } catch {
            alertMessage = error.localizedDescription
            showAlert = true
        }
    }
}

struct MatchCard: View {
    let match: SwapMatch
    let onPropose: () -> Void

    var body: some View {
        VStack(spacing: 14) {
            // Header: Reason & Score
            HStack {
                Text("🔥 Perfekter Match")
                    .font(.caption.bold())
                    .padding(.horizontal, 10)
                    .padding(.vertical, 4)
                    .background(Color.orange.opacity(0.15))
                    .foregroundColor(.orange)
                    .clipShape(Capsule())

                Spacer()

                Text("\(match.matchScore)% Match")
                    .font(.caption.bold())
                    .foregroundColor(.secondary)
            }

            // Side-by-Side: You give vs You get
            HStack(spacing: 12) {
                // My Item
                VStack(alignment: .leading, spacing: 4) {
                    Text("DU GIBST:")
                        .font(.caption2.bold())
                        .foregroundColor(.secondary)
                    Text(match.myItem.title)
                        .font(.subheadline.bold())
                        .lineLimit(2)
                    Text("Wert: \(Int(match.myItem.estimatedValue)) €")
                        .font(.caption)
                        .foregroundColor(.blue)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(10)
                .background(Color(.tertiarySystemGroupedBackground))
                .clipShape(RoundedRectangle(cornerRadius: 12))

                Image(systemName: "arrow.left.arrow.right")
                    .foregroundColor(.orange)
                    .font(.headline)

                // Their Item
                VStack(alignment: .leading, spacing: 4) {
                    Text("DU ERHÄLTST:")
                        .font(.caption2.bold())
                        .foregroundColor(.secondary)
                    Text(match.theirItem.title)
                        .font(.subheadline.bold())
                        .lineLimit(2)
                    Text("Wert: \(Int(match.theirItem.estimatedValue)) €")
                        .font(.caption)
                        .foregroundColor(.green)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(10)
                .background(Color(.tertiarySystemGroupedBackground))
                .clipShape(RoundedRectangle(cornerRadius: 12))
            }

            // Partner Info & Propose Button
            HStack {
                HStack(spacing: 6) {
                    Image(systemName: "person.circle.fill")
                        .foregroundColor(.secondary)
                    Text("@\(match.otherUser.username)")
                        .font(.caption.bold())
                }

                Spacer()

                Button(action: onPropose) {
                    Text("Tausch vorschlagen")
                        .font(.caption.bold())
                        .padding(.horizontal, 14)
                        .padding(.vertical, 8)
                        .background(Color.blue)
                        .foregroundColor(.white)
                        .clipShape(Capsule())
                }
            }
        }
        .padding()
        .background(Color(.secondarySystemGroupedBackground))
        .clipShape(RoundedRectangle(cornerRadius: 16))
        .shadow(color: .black.opacity(0.04), radius: 6, y: 2)
    }
}
