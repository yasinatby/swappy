import Foundation

public struct User: Codable, Identifiable, Sendable {
    public let id: String
    public let username: String
    public let email: String?
    public let firstName: String?
    public let avatarUrl: String?
    public let bio: String?
    public let city: String?
    public let postalCode: String?
    public let latitude: Double?
    public let longitude: Double?
    public let isVerified: Bool
    public let isBanned: Bool
    public let role: String
    public let completedTradesCount: Int
    public let ratingAverage: Double
    public let ratingCount: Int
    public let createdAt: String?
    public let updatedAt: String?

    enum CodingKeys: String, CodingKey {
        case id
        case username
        case email
        case firstName = "first_name"
        case avatarUrl = "avatar_url"
        case bio
        case city
        case postalCode = "postal_code"
        case latitude
        case longitude
        case isVerified = "is_verified"
        case isBanned = "is_banned"
        case role
        case completedTradesCount = "completed_trades_count"
        case ratingAverage = "rating_average"
        case ratingCount = "rating_count"
        case createdAt = "created_at"
        case updatedAt = "updated_at"
    }
}

public struct UserPublicProfile: Codable, Identifiable, Sendable {
    public let id: String
    public let username: String
    public let firstName: String?
    public let avatarUrl: String?
    public let bio: String?
    public let city: String?
    public let completedTradesCount: Int
    public let ratingAverage: Double
    public let ratingCount: Int
    public let isVerified: Bool
    public let createdAt: String?
    public let activeItems: [Item]?
    public let tradedItems: [Item]?

    enum CodingKeys: String, CodingKey {
        case id
        case username
        case firstName = "first_name"
        case avatarUrl = "avatar_url"
        case bio
        case city
        case completedTradesCount = "completed_trades_count"
        case ratingAverage = "rating_average"
        case ratingCount = "rating_count"
        case isVerified = "is_verified"
        case createdAt = "created_at"
        case activeItems = "active_items"
        case tradedItems = "traded_items"
    }
}

public struct AuthResponse: Codable, Sendable {
    public let token: String
    public let user: User
}

public struct Item: Codable, Identifiable, Sendable {
    public let id: String
    public let userId: String
    public let title: String
    public let description: String
    public let category: String
    public let condition: String
    public let estimatedValue: Double
    public let minSwapValue: Double?
    public let tradeInterests: [String]
    public let images: [String]
    public let city: String?
    public let latitude: Double?
    public let longitude: Double?
    public let status: String // available, requested, traded, deactivated
    public let viewsCount: Int
    public let favoritesCount: Int
    public let isFavorited: Bool?
    public let owner: UserPublicProfile?
    public let createdAt: String?
    public let updatedAt: String?

    enum CodingKeys: String, CodingKey {
        case id
        case userId = "user_id"
        case title
        case description
        case category
        case condition
        case estimatedValue = "estimated_value"
        case minSwapValue = "min_swap_value"
        case tradeInterests = "trade_interests"
        case images
        case city
        case latitude
        case longitude
        case status
        case viewsCount = "views_count"
        case favoritesCount = "favorites_count"
        case isFavorited = "is_favorited"
        case owner
        case createdAt = "created_at"
        case updatedAt = "updated_at"
    }
}

public struct SwapHistory: Codable, Identifiable, Sendable {
    public let id: String
    public let swapId: String
    public let actorId: String
    public let actorUsername: String?
    public let action: String
    public let message: String?
    public let cashAdjustment: Double
    public let offeredItemIds: [String]?
    public let createdAt: String?

    enum CodingKeys: String, CodingKey {
        case id
        case swapId = "swap_id"
        case actorId = "actor_id"
        case actorUsername = "actor_username"
        case action
        case message
        case cashAdjustment = "cash_adjustment"
        case offeredItemIds = "offered_item_ids"
        case createdAt = "created_at"
    }
}

public struct Swap: Codable, Identifiable, Sendable {
    public let id: String
    public let initiatorId: String
    public let recipientId: String
    public let targetItemId: String
    public let targetItem: Item?
    public let offeredItems: [Item]?
    public let status: String // pending, countered, agreed, completed, rejected, cancelled
    public let cashAdjustment: Double
    public let message: String?
    public let handoverMethod: String // personal, shipping
    public let initiatorConfirmed: Bool
    public let recipientConfirmed: Bool
    public let initiator: UserPublicProfile?
    public let recipient: UserPublicProfile?
    public let history: [SwapHistory]?
    public let createdAt: String?
    public let updatedAt: String?

    enum CodingKeys: String, CodingKey {
        case id
        case initiatorId = "initiator_id"
        case recipientId = "recipient_id"
        case targetItemId = "target_item_id"
        case targetItem = "target_item"
        case offeredItems = "offered_items"
        case status
        case cashAdjustment = "cash_adjustment"
        case message
        case handoverMethod = "handover_method"
        case initiatorConfirmed = "initiator_confirmed"
        case recipientConfirmed = "recipient_confirmed"
        case initiator
        case recipient
        case history
        case createdAt = "created_at"
        case updatedAt = "updated_at"
    }
}

public struct Message: Codable, Identifiable, Sendable {
    public let id: String
    public let swapId: String
    public let senderId: String
    public let sender: UserPublicProfile?
    public let text: String?
    public let imageUrl: String?
    public let isSystem: Bool
    public let createdAt: String?

    enum CodingKeys: String, CodingKey {
        case id
        case swapId = "swap_id"
        case senderId = "sender_id"
        case sender
        case text
        case imageUrl = "image_url"
        case isSystem = "is_system"
        case createdAt = "created_at"
    }
}

public struct Review: Codable, Identifiable, Sendable {
    public let id: String
    public let swapId: String
    public let reviewerId: String
    public let targetId: String
    public let rating: Int
    public let reliability: Int
    public let communication: Int
    public let itemCondition: Int
    public let adherence: Int
    public let comment: String
    public let reviewer: UserPublicProfile?
    public let createdAt: String?

    enum CodingKeys: String, CodingKey {
        case id
        case swapId = "swap_id"
        case reviewerId = "reviewer_id"
        case targetId = "target_id"
        case rating
        case reliability
        case communication
        case itemCondition = "item_condition"
        case adherence
        case comment
        case reviewer
        case createdAt = "created_at"
    }
}

public struct Notification: Codable, Identifiable, Sendable {
    public let id: String
    public let userId: String
    public let type: String
    public let title: String
    public let body: String
    public let entityType: String?
    public let entityId: String?
    public let isRead: Bool
    public let createdAt: String?

    enum CodingKeys: String, CodingKey {
        case id
        case userId = "user_id"
        case type
        case title
        case body
        case entityType = "entity_type"
        case entityId = "entity_id"
        case isRead = "is_read"
        case createdAt = "created_at"
    }
}

public struct SwapMatch: Codable, Identifiable, Sendable {
    public var id: String { myItem.id + "_" + theirItem.id }
    public let myItem: Item
    public let theirItem: Item
    public let otherUser: UserPublicProfile
    public let matchScore: Int
    public let matchReason: String
    public let estimatedDiff: Double

    enum CodingKeys: String, CodingKey {
        case myItem = "my_item"
        case theirItem = "their_item"
        case otherUser = "other_user"
        case matchScore = "match_score"
        case matchReason = "match_reason"
        case estimatedDiff = "estimated_diff"
    }
}

public struct CategoryInfo: Codable, Sendable {
    public let name: String
    public let itemCount: Int

    enum CodingKeys: String, CodingKey {
        case name
        case itemCount = "item_count"
    }
}

public struct HomeFeed: Codable, Sendable {
    public let categories: [CategoryInfo]
    public let recentItems: [Item]
    public let nearbyItems: [Item]
    public let popularItems: [Item]

    enum CodingKeys: String, CodingKey {
        case categories
        case recentItems = "recent_items"
        case nearbyItems = "nearby_items"
        case popularItems = "popular_items"
    }
}
