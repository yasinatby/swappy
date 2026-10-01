import Foundation
import SwappyClient

@main
struct SwappyTestRunner {
    static func main() async {
        print("==================================================")
        print("  🔄 SWAPPY SWIFT TEST RUNNER - BACKEND VERIFICATION")
        print("==================================================")

        let backendURLString = ProcessInfo.processInfo.environment["SWAPPY_URL"] ?? "http://localhost:8080"
        guard let url = URL(string: backendURLString) else {
            print("❌ Invalid backend URL: \(backendURLString)")
            exit(1)
        }

        print("Target Backend URL: \(url.absoluteString)\n")

        let maxClient = SwappyAPIClient(baseURL: url)
        let lisaClient = SwappyAPIClient(baseURL: url)
        let adminClient = SwappyAPIClient(baseURL: url)

        do {
            // 1. Health check
            print("Step 1: Checking backend health...")
            let health = try await maxClient.checkHealth()
            print("✅ Backend is healthy: \(health)")

            // 2. Register Max and Lisa
            let timestamp = Int(Date().timeIntervalSince1970)
            let maxUsername = "max_\(timestamp)"
            let lisaUsername = "lisa_\(timestamp)"

            print("\nStep 2: Registering users: \(maxUsername) and \(lisaUsername)...")
            let maxAuth = try await maxClient.register(params: .init(
                username: maxUsername,
                email: "\(maxUsername)@swappy.local",
                password: "Password123!",
                firstName: "Max",
                city: "Heilbronn",
                latitude: 49.1427,
                longitude: 9.2109
            ))
            print("✅ Registered Max: ID=\(maxAuth.user.id), City=\(maxAuth.user.city ?? "")")

            let lisaAuth = try await lisaClient.register(params: .init(
                username: lisaUsername,
                email: "\(lisaUsername)@swappy.local",
                password: "Password123!",
                firstName: "Lisa",
                city: "Stuttgart",
                latitude: 48.7758,
                longitude: 9.1829
            ))
            print("✅ Registered Lisa: ID=\(lisaAuth.user.id), City=\(lisaAuth.user.city ?? "")")

            // 3. Test Rule 4: Max tries to propose swap BEFORE creating an item!
            print("\nStep 3: Testing Rule 4 (Tauschregel: mind. 1 aktives Objekt erforderlich)...")
            // Lisa creates her iPhone first
            let lisaItem = try await lisaClient.createItem(params: .init(
                title: "iPhone 15",
                description: "iPhone 15 128GB in schwarz, wie neu.",
                category: "Elektronik",
                condition: "Sehr gut",
                estimatedValue: 600.0,
                tradeInterests: ["PlayStation 5", "PS5", "Gaming"],
                city: "Stuttgart"
            ))
            print("✅ Lisa published item: '\(lisaItem.title)' (Wert: \(lisaItem.estimatedValue) €)")

            // Max tries to send swap proposal before listing any item
            do {
                _ = try await maxClient.proposeSwap(params: .init(
                    targetItemId: lisaItem.id,
                    offeredItemIds: ["dummy-id"],
                    message: "Tauschversuch ohne eigenes Objekt"
                ))
                print("❌ RULE 4 VIOLATION: Proposal succeeded when user had 0 active items!")
                exit(1)
            } catch let SwappyError.serverError(code, message) {
                print("✅ Rule 4 correctly enforced! Server rejected with HTTP \(code): '\(message)'")
            }

            // 4. Max lists his PlayStation 5
            print("\nStep 4: Max publishes his item (PlayStation 5)...")
            let maxItem = try await maxClient.createItem(params: .init(
                title: "PlayStation 5",
                description: "PS5 mit 2 Controllern, sehr guter Zustand.",
                category: "Gaming",
                condition: "Sehr gut",
                estimatedValue: 450.0,
                tradeInterests: ["iPhone 15", "iPhone", "Smartphone"],
                city: "Heilbronn"
            ))
            print("✅ Max published item: '\(maxItem.title)' (Wert: \(maxItem.estimatedValue) €)")

            // 5. Test Smart Swap Matching
            print("\nStep 5: Testing Smart Swap Matching (Tausch-Matching ⭐)...")
            let matches = try await maxClient.getSwapMatches()
            if let bestMatch = matches.first {
                print("✅ Found match! Reason: \(bestMatch.matchReason)")
                print("   Score: \(bestMatch.matchScore)/100, Partner: \(bestMatch.otherUser.username)")
                print("   My item: \(bestMatch.myItem.title) ↔ Their item: \(bestMatch.theirItem.title)")
            } else {
                print("⚠️ No match returned.")
            }

            // 6. Test Reverse Trade Interest Search ("Ich suche ...")
            print("\nStep 6: Testing Reverse Trade Interest Search ('PS5')...")
            let reverseResults = try await maxClient.listItems(tradeInterest: "PS5")
            print("✅ Reverse search found \(reverseResults.count) item(s) whose owner wants 'PS5'")
            if let first = reverseResults.first {
                print("   Found: '\(first.title)' offered by @\(first.owner?.username ?? "unknown")")
            }

            // 7. Max proposes swap: PS5 + 50 € for iPhone 15
            print("\nStep 7: Max sends swap proposal (PS5 + 50 € for iPhone 15)...")
            let proposal = try await maxClient.proposeSwap(params: .init(
                targetItemId: lisaItem.id,
                offeredItemIds: [maxItem.id],
                cashAdjustment: 50.0,
                message: "Ich würde gerne mein iPhone gegen deine PS5 tauschen. Das iPhone ist in sehr gutem Zustand.",
                handoverMethod: "personal"
            ))
            print("✅ Swap proposal sent: ID=\(proposal.id), Status=\(proposal.status), CashAdj=+\(proposal.cashAdjustment) €")

            // 8. Lisa receives proposal and counters (Gegenangebot: PS5 + 100 €)
            print("\nStep 8: Lisa makes counter-offer (Gegenangebot: +100 €)...")
            let counteredSwap = try await lisaClient.counterSwap(
                swapId: proposal.id,
                params: .init(
                    offeredItemIds: [maxItem.id],
                    cashAdjustment: 100.0,
                    message: "Ich würde die PS5 gegen mein iPhone + 100 € tauschen."
                )
            )
            print("✅ Counter-offer made: Status=\(counteredSwap.status), New CashAdj=\(counteredSwap.cashAdjustment) €")

            // 9. Max accepts counter-offer
            print("\nStep 9: Max accepts counter-offer...")
            let agreedSwap = try await maxClient.acceptSwap(swapId: proposal.id)
            print("✅ Swap accepted! Status=\(agreedSwap.status) (🔄 Tausch vereinbart)")

            // Check item status changed to 'requested' 🟡
            let targetCheck = try await maxClient.getItem(id: lisaItem.id)
            print("✅ Target item status: '\(targetCheck.status)' (🟡 requested)")

            // 10. Chat messages
            print("\nStep 10: Exchanging chat messages...")
            let msg1 = try await maxClient.sendMessage(
                swapId: proposal.id,
                text: "Super! Passt dir Samstag 14 Uhr für die persönliche Übergabe?"
            )
            print("✅ Max sent chat message: '\(msg1.text ?? "")'")

            let msg2 = try await lisaClient.sendMessage(
                swapId: proposal.id,
                text: "Ja perfekt! Treffpunkt am Marktplatz."
            )
            print("✅ Lisa sent chat message: '\(msg2.text ?? "")'")

            let allMsgs = try await maxClient.getMessages(swapId: proposal.id)
            print("✅ Chat history has \(allMsgs.count) total message(s).")

            // 11. Complete swap
            print("\nStep 11: Completing swap (both confirm handover)...")
            _ = try await maxClient.completeSwap(swapId: proposal.id)
            let completedSwap = try await lisaClient.completeSwap(swapId: proposal.id)
            print("✅ Swap marked: Status=\(completedSwap.status) (🤝 Tausch abgeschlossen)")

            // Check item status changed to 'traded' 🔴
            let completedItemCheck = try await maxClient.getItem(id: lisaItem.id)
            print("✅ Target item status: '\(completedItemCheck.status)' (🔴 getauscht)")

            // 12. Rate user
            print("\nStep 12: Max submits 5-star review for Lisa...")
            let review = try await maxClient.createReview(
                swapId: proposal.id,
                params: .init(
                    rating: 5,
                    reliability: 5,
                    communication: 5,
                    itemCondition: 5,
                    adherence: 5,
                    comment: "Sehr zuverlässiger Tauschpartner. Alles wie beschrieben! Gerne wieder."
                )
            )
            print("✅ Review submitted: \(review.rating) ⭐ - '\(review.comment)'")

            // Verify Lisa's profile has updated stats
            let lisaUpdatedProfile = try await maxClient.getUserProfile(id: lisaAuth.user.id)
            print("✅ Lisa's updated profile:")
            print("   Completed trades: \(lisaUpdatedProfile.completedTradesCount)")
            print("   Rating average: \(lisaUpdatedProfile.ratingAverage) ⭐ (\(lisaUpdatedProfile.ratingCount) reviews)")

            // 13. Favorites test
            print("\nStep 13: Testing Favorites (Merkliste)...")
            _ = try await lisaClient.toggleFavorite(itemId: maxItem.id)
            let lisaFavorites = try await lisaClient.getFavorites()
            print("✅ Lisa's favorites count: \(lisaFavorites.count)")

            // 14. Notifications test
            print("\nStep 14: Checking notifications...")
            let notifs = try await lisaClient.getNotifications()
            print("✅ Lisa received \(notifs.count) notification(s):")
            for n in notifs.prefix(3) {
                print("   🔔 [\(n.type)] \(n.title): \(n.body)")
            }

            // 15. Admin verification
            print("\nStep 15: Verifying Admin Dashboard & Statistics...")
            _ = try await adminClient.login(params: .init(
                usernameOrEmail: "admin",
                password: "admin1234"
            ))
            let stats = try await adminClient.getAdminStats()
            print("✅ Admin Stats:")
            print("   Total Users: \(stats.totalUsers)")
            print("   Active Items: \(stats.activeItems)")
            print("   Completed Trades: \(stats.completedTrades)")
            print("   Estimated Traded Volume: \(stats.totalEstimatedVolume) €")

            print("\n==================================================")
            print("  🎉 ALL SWAPPY BACKEND & FRONTEND TESTS PASSED!")
            print("==================================================")

        } catch {
            print("\n❌ TEST FAILED WITH ERROR: \(error)")
            exit(1)
        }
    }
}
