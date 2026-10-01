import Testing
import Foundation
@testable import SwappyClient

@Suite("Swappy Client Tests")
struct SwappyClientTests {

    @Test("Test User Public Profile Decoding")
    func testUserPublicProfileDecoding() throws {
        let json = """
        {
            "id": "u-123",
            "username": "max",
            "first_name": "Max",
            "avatar_url": "https://example.com/avatar.jpg",
            "city": "Heilbronn",
            "completed_trades_count": 5,
            "rating_average": 4.8,
            "rating_count": 12,
            "is_verified": true,
            "created_at": "2026-09-08T12:00:00Z"
        }
        """.data(using: .utf8)!

        let profile = try JSONDecoder().decode(UserPublicProfile.self, from: json)
        #expect(profile.id == "u-123")
        #expect(profile.username == "max")
        #expect(profile.completedTradesCount == 5)
        #expect(profile.ratingAverage == 4.8)
        #expect(profile.isVerified == true)
    }

    @Test("Test Item Decoding")
    func testItemDecoding() throws {
        let json = """
        {
            "id": "item-456",
            "user_id": "u-123",
            "title": "PlayStation 5",
            "description": "PS5 mit 2 Controllern",
            "category": "Gaming",
            "condition": "Sehr gut",
            "estimated_value": 450.0,
            "trade_interests": ["iPhone 15", "Gaming-PC"],
            "images": ["/uploads/ps5.jpg"],
            "city": "Heilbronn",
            "status": "available",
            "views_count": 42,
            "favorites_count": 3
        }
        """.data(using: .utf8)!

        let item = try JSONDecoder().decode(Item.self, from: json)
        #expect(item.title == "PlayStation 5")
        #expect(item.category == "Gaming")
        #expect(item.estimatedValue == 450.0)
        #expect(item.tradeInterests.count == 2)
        #expect(item.status == "available")
    }

    @Test("Test Swap Match Decoding")
    func testSwapMatchDecoding() throws {
        let json = """
        {
            "my_item": {
                "id": "item-1",
                "user_id": "u-1",
                "title": "PlayStation 5",
                "description": "PS5",
                "category": "Gaming",
                "condition": "Sehr gut",
                "estimated_value": 450.0,
                "trade_interests": ["iPhone"],
                "images": [],
                "status": "available",
                "views_count": 10,
                "favorites_count": 1
            },
            "their_item": {
                "id": "item-2",
                "user_id": "u-2",
                "title": "iPhone 15",
                "description": "iPhone",
                "category": "Elektronik",
                "condition": "Wie neu",
                "estimated_value": 600.0,
                "trade_interests": ["PS5"],
                "images": [],
                "status": "available",
                "views_count": 25,
                "favorites_count": 4
            },
            "other_user": {
                "id": "u-2",
                "username": "lisa",
                "completed_trades_count": 3,
                "rating_average": 5.0,
                "rating_count": 3,
                "is_verified": true
            },
            "match_score": 100,
            "match_reason": "Perfekter Tausch-Match",
            "estimated_diff": 150.0
        }
        """.data(using: .utf8)!

        let match = try JSONDecoder().decode(SwapMatch.self, from: json)
        #expect(match.matchScore == 100)
        #expect(match.myItem.title == "PlayStation 5")
        #expect(match.theirItem.title == "iPhone 15")
        #expect(match.otherUser.username == "lisa")
    }
}
