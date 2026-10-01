import Foundation

/// Every backend call the app makes. Views never talk to this directly; they
/// go through their view model.
struct SwappyAPI {
    let client: APIClient

    func imageURL(_ path: String?) -> URL? {
        path.map(client.imageURL)
    }

    // MARK: Dev login (only while real sign-in is not built)

    func devProfiles() async throws -> [Profile] {
        let response: ProfilesResponse = try await client.get("v1/dev/profiles")
        return response.profiles
    }

    func devLogin(displayName: String) async throws -> DevLoginResponse {
        try await client.post("v1/dev/login", body: ["display_name": displayName])
    }

    // MARK: Me

    func me() async throws -> Profile {
        try await client.get("v1/me")
    }

    func updateMe(displayName: String, postalCode: String) async throws -> Profile {
        try await client.put("v1/me", body: ["display_name": displayName, "postal_code": postalCode])
    }

    func deleteAccount() async throws {
        try await client.perform("DELETE", "v1/me")
    }

    func myItems() async throws -> [Item] {
        let response: ItemsResponse = try await client.get("v1/me/items")
        return response.items
    }

    func blockedUsers() async throws -> [BlockedUser] {
        let response: BlockedResponse = try await client.get("v1/me/blocks")
        return response.blocked
    }

    // MARK: Items

    func feed(search: String = "", category: String? = nil, ownerId: String? = nil, offset: Int = 0) async throws -> [Item] {
        let response: ItemsResponse = try await client.get("v1/items", query: [
            "q": search, "category": category ?? "", "owner": ownerId ?? "", "offset": offset > 0 ? String(offset) : "",
        ])
        return response.items
    }

    func item(_ id: String) async throws -> Item {
        try await client.get("v1/items/\(id)")
    }

    func createItem(_ input: ItemInput, publish: Bool) async throws -> Item {
        struct Body: Encodable {
            let title: String, description: String?, category: String
            let condition: ItemCondition, wantedText: String?, publish: Bool
        }
        return try await client.post("v1/items", body: Body(
            title: input.title, description: input.description, category: input.category,
            condition: input.condition, wantedText: input.wantedText, publish: publish))
    }

    func updateItem(_ id: String, _ input: ItemInput) async throws -> Item {
        try await client.put("v1/items/\(id)", body: input)
    }

    func publishItem(_ id: String) async throws -> Item {
        try await client.post("v1/items/\(id)/publish")
    }

    func deleteItem(_ id: String) async throws {
        try await client.perform("DELETE", "v1/items/\(id)")
    }

    func uploadImage(itemId: String, jpeg: Data) async throws -> ItemImage {
        try await client.upload("v1/items/\(itemId)/images", jpeg: jpeg)
    }

    func deleteImage(itemId: String, imageId: String) async throws {
        try await client.perform("DELETE", "v1/items/\(itemId)/images/\(imageId)")
    }

    // MARK: Users, blocks, reports

    func user(_ id: String) async throws -> Profile {
        try await client.get("v1/users/\(id)")
    }

    func block(_ userId: String) async throws {
        try await client.perform("POST", "v1/users/\(userId)/block")
    }

    func unblock(_ userId: String) async throws {
        try await client.perform("DELETE", "v1/users/\(userId)/block")
    }

    func report(_ subject: ReportSubject, id: String, reason: ReportReason, note: String?) async throws {
        struct Body: Encodable {
            let subjectType: ReportSubject, subjectId: String, reason: ReportReason, note: String?
        }
        try await client.perform("POST", "v1/reports", body: Body(subjectType: subject, subjectId: id, reason: reason, note: note))
    }

    // MARK: Offers

    func offers() async throws -> [Offer] {
        let response: OffersResponse = try await client.get("v1/offers")
        return response.offers
    }

    func offer(_ id: String) async throws -> Offer {
        try await client.get("v1/offers/\(id)")
    }

    func createOffer(_ input: CreateOfferInput) async throws -> Offer {
        try await client.post("v1/offers", body: input)
    }

    func perform(_ action: OfferAction, offerId: String) async throws -> Offer {
        try await client.post("v1/offers/\(offerId)/\(action.rawValue)")
    }

    // MARK: Notifications

    func notifications() async throws -> [AppNotification] {
        let response: NotificationsResponse = try await client.get("v1/notifications")
        return response.notifications
    }

    func markNotificationsRead() async throws {
        try await client.perform("POST", "v1/notifications/read")
    }
}
