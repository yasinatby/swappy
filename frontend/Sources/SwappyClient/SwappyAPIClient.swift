import Foundation

public enum SwappyError: Error, LocalizedError {
    case invalidURL
    case serverError(statusCode: Int, message: String)
    case decodingError(Error)
    case networkError(Error)

    public var errorDescription: String? {
        switch self {
        case .invalidURL:
            return "Invalid URL"
        case .serverError(let code, let msg):
            return "Server Error (\(code)): \(msg)"
        case .decodingError(let err):
            return "Failed to decode response: \(err.localizedDescription)"
        case .networkError(let err):
            return "Network Error: \(err.localizedDescription)"
        }
    }
}

public actor SwappyAPIClient {
    public let baseURL: URL
    public var token: String?

    private let session: URLSession
    private let decoder: JSONDecoder
    private let encoder: JSONEncoder

    public init(baseURL: URL, token: String? = nil, session: URLSession = .shared) {
        self.baseURL = baseURL
        self.token = token
        self.session = session
        self.decoder = JSONDecoder()
        self.encoder = JSONEncoder()
    }

    public func setToken(_ newToken: String?) {
        self.token = newToken
    }

    // MARK: - Generic Request

    private func sendRequest<T: Decodable>(
        method: String,
        path: String,
        queryItems: [URLQueryItem]? = nil,
        body: (any Encodable)? = nil
    ) async throws -> T {
        var components = URLComponents(url: baseURL.appendingPathComponent(path), resolvingAgainstBaseURL: true)
        if let queryItems = queryItems, !queryItems.isEmpty {
            components?.queryItems = queryItems
        }

        guard let url = components?.url else {
            throw SwappyError.invalidURL
        }

        var request = URLRequest(url: url)
        request.httpMethod = method
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")

        if let token = self.token {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }

        if let body = body {
            do {
                request.httpBody = try encoder.encode(body)
            } catch {
                throw SwappyError.networkError(error)
            }
        }

        let (data, response): (Data, URLResponse)
        do {
            (data, response) = try await session.data(for: request)
        } catch {
            throw SwappyError.networkError(error)
        }

        guard let httpResponse = response as? HTTPURLResponse else {
            throw SwappyError.serverError(statusCode: 0, message: "Non-HTTP response")
        }

        if httpResponse.statusCode < 200 || httpResponse.statusCode >= 300 {
            if let errorObj = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
               let errMsg = errorObj["error"] as? String {
                throw SwappyError.serverError(statusCode: httpResponse.statusCode, message: errMsg)
            }
            let rawStr = String(data: data, encoding: .utf8) ?? "Unknown error"
            throw SwappyError.serverError(statusCode: httpResponse.statusCode, message: rawStr)
        }

        do {
            return try decoder.decode(T.self, from: data)
        } catch {
            throw SwappyError.decodingError(error)
        }
    }

    // MARK: - Health Check

    public func checkHealth() async throws -> [String: String] {
        return try await sendRequest(method: "GET", path: "/health")
    }

    // MARK: - Auth

    public struct RegisterParams: Encodable, Sendable {
        public let username: String
        public let email: String
        public let password: String
        public let firstName: String?
        public let city: String?
        public let latitude: Double?
        public let longitude: Double?

        public init(username: String, email: String, password: String, firstName: String? = nil, city: String? = nil, latitude: Double? = nil, longitude: Double? = nil) {
            self.username = username
            self.email = email
            self.password = password
            self.firstName = firstName
            self.city = city
            self.latitude = latitude
            self.longitude = longitude
        }

        enum CodingKeys: String, CodingKey {
            case username, email, password
            case firstName = "first_name"
            case city, latitude, longitude
        }
    }

    public struct LoginParams: Encodable, Sendable {
        public let usernameOrEmail: String
        public let password: String

        public init(usernameOrEmail: String, password: String) {
            self.usernameOrEmail = usernameOrEmail
            self.password = password
        }

        enum CodingKeys: String, CodingKey {
            case usernameOrEmail = "username_or_email"
            case password
        }
    }

    public func register(params: RegisterParams) async throws -> AuthResponse {
        let auth: AuthResponse = try await sendRequest(method: "POST", path: "/api/auth/register", body: params)
        self.token = auth.token
        return auth
    }

    public func login(params: LoginParams) async throws -> AuthResponse {
        let auth: AuthResponse = try await sendRequest(method: "POST", path: "/api/auth/login", body: params)
        self.token = auth.token
        return auth
    }

    public func getMe() async throws -> User {
        return try await sendRequest(method: "GET", path: "/api/auth/me")
    }

    public func getUserProfile(id: String) async throws -> UserPublicProfile {
        return try await sendRequest(method: "GET", path: "/api/users/\(id)")
    }

    // MARK: - Items

    public struct CreateItemParams: Encodable, Sendable {
        public let title: String
        public let description: String
        public let category: String
        public let condition: String
        public let estimatedValue: Double
        public let minSwapValue: Double?
        public let tradeInterests: [String]
        public let images: [String]
        public let city: String?

        public init(
            title: String,
            description: String,
            category: String,
            condition: String,
            estimatedValue: Double,
            minSwapValue: Double? = nil,
            tradeInterests: [String] = [],
            images: [String] = [],
            city: String? = nil
        ) {
            self.title = title
            self.description = description
            self.category = category
            self.condition = condition
            self.estimatedValue = estimatedValue
            self.minSwapValue = minSwapValue
            self.tradeInterests = tradeInterests
            self.images = images
            self.city = city
        }

        enum CodingKeys: String, CodingKey {
            case title, description, category, condition
            case estimatedValue = "estimated_value"
            case minSwapValue = "min_swap_value"
            case tradeInterests = "trade_interests"
            case images, city
        }
    }

    public func createItem(params: CreateItemParams) async throws -> Item {
        return try await sendRequest(method: "POST", path: "/api/items", body: params)
    }

    public func getItem(id: String) async throws -> Item {
        return try await sendRequest(method: "GET", path: "/api/items/\(id)")
    }

    public func listItems(
        query: String? = nil,
        category: String? = nil,
        tradeInterest: String? = nil,
        status: String? = nil,
        sort: String? = nil
    ) async throws -> [Item] {
        var qItems: [URLQueryItem] = []
        if let query = query { qItems.append(URLQueryItem(name: "q", value: query)) }
        if let category = category { qItems.append(URLQueryItem(name: "category", value: category)) }
        if let tradeInterest = tradeInterest { qItems.append(URLQueryItem(name: "trade_interest", value: tradeInterest)) }
        if let status = status { qItems.append(URLQueryItem(name: "status", value: status)) }
        if let sort = sort { qItems.append(URLQueryItem(name: "sort", value: sort)) }

        return try await sendRequest(method: "GET", path: "/api/items", queryItems: qItems)
    }

    public func toggleFavorite(itemId: String) async throws -> [String: AnyCodable] {
        return try await sendRequest(method: "POST", path: "/api/items/\(itemId)/favorite")
    }

    public func getFavorites() async throws -> [Item] {
        return try await sendRequest(method: "GET", path: "/api/favorites")
    }

    public func getHomeFeed(latitude: Double? = nil, longitude: Double? = nil) async throws -> HomeFeed {
        var qItems: [URLQueryItem] = []
        if let lat = latitude { qItems.append(URLQueryItem(name: "lat", value: "\(lat)")) }
        if let lng = longitude { qItems.append(URLQueryItem(name: "lng", value: "\(lng)")) }

        return try await sendRequest(method: "GET", path: "/api/feed/home", queryItems: qItems)
    }

    // MARK: - Swaps

    public struct ProposeSwapParams: Encodable, Sendable {
        public let targetItemId: String
        public let offeredItemIds: [String]
        public let cashAdjustment: Double
        public let message: String
        public let handoverMethod: String

        public init(
            targetItemId: String,
            offeredItemIds: [String],
            cashAdjustment: Double = 0.0,
            message: String = "",
            handoverMethod: String = "personal"
        ) {
            self.targetItemId = targetItemId
            self.offeredItemIds = offeredItemIds
            self.cashAdjustment = cashAdjustment
            self.message = message
            self.handoverMethod = handoverMethod
        }

        enum CodingKeys: String, CodingKey {
            case targetItemId = "target_item_id"
            case offeredItemIds = "offered_item_ids"
            case cashAdjustment = "cash_adjustment"
            case message
            case handoverMethod = "handover_method"
        }
    }

    public struct CounterSwapParams: Encodable, Sendable {
        public let offeredItemIds: [String]
        public let cashAdjustment: Double
        public let message: String

        public init(offeredItemIds: [String], cashAdjustment: Double, message: String) {
            self.offeredItemIds = offeredItemIds
            self.cashAdjustment = cashAdjustment
            self.message = message
        }

        enum CodingKeys: String, CodingKey {
            case offeredItemIds = "offered_item_ids"
            case cashAdjustment = "cash_adjustment"
            case message
        }
    }

    public func proposeSwap(params: ProposeSwapParams) async throws -> Swap {
        return try await sendRequest(method: "POST", path: "/api/swaps", body: params)
    }

    public func counterSwap(swapId: String, params: CounterSwapParams) async throws -> Swap {
        return try await sendRequest(method: "POST", path: "/api/swaps/\(swapId)/counter", body: params)
    }

    public func acceptSwap(swapId: String) async throws -> Swap {
        return try await sendRequest(method: "POST", path: "/api/swaps/\(swapId)/accept")
    }

    public func rejectSwap(swapId: String) async throws -> Swap {
        return try await sendRequest(method: "POST", path: "/api/swaps/\(swapId)/reject")
    }

    public func completeSwap(swapId: String) async throws -> Swap {
        return try await sendRequest(method: "POST", path: "/api/swaps/\(swapId)/complete")
    }

    public func getSwap(swapId: String) async throws -> Swap {
        return try await sendRequest(method: "GET", path: "/api/swaps/\(swapId)")
    }

    public func listSwaps(status: String? = nil) async throws -> [Swap] {
        var qItems: [URLQueryItem] = []
        if let s = status { qItems.append(URLQueryItem(name: "status", value: s)) }
        return try await sendRequest(method: "GET", path: "/api/swaps", queryItems: qItems)
    }

    public func getSwapMatches() async throws -> [SwapMatch] {
        return try await sendRequest(method: "GET", path: "/api/swaps/matches")
    }

    // MARK: - Chat

    public struct SendMessageParams: Encodable, Sendable {
        public let text: String?
        public let imageUrl: String?

        public init(text: String? = nil, imageUrl: String? = nil) {
            self.text = text
            self.imageUrl = imageUrl
        }

        enum CodingKeys: String, CodingKey {
            case text
            case imageUrl = "image_url"
        }
    }

    public func getMessages(swapId: String) async throws -> [Message] {
        return try await sendRequest(method: "GET", path: "/api/swaps/\(swapId)/messages")
    }

    public func sendMessage(swapId: String, text: String) async throws -> Message {
        return try await sendRequest(
            method: "POST",
            path: "/api/swaps/\(swapId)/messages",
            body: SendMessageParams(text: text)
        )
    }

    // MARK: - Reviews

    public struct CreateReviewParams: Encodable, Sendable {
        public let rating: Int
        public let reliability: Int
        public let communication: Int
        public let itemCondition: Int
        public let adherence: Int
        public let comment: String

        public init(
            rating: Int,
            reliability: Int = 5,
            communication: Int = 5,
            itemCondition: Int = 5,
            adherence: Int = 5,
            comment: String
        ) {
            self.rating = rating
            self.reliability = reliability
            self.communication = communication
            self.itemCondition = itemCondition
            self.adherence = adherence
            self.comment = comment
        }

        enum CodingKeys: String, CodingKey {
            case rating, reliability, communication
            case itemCondition = "item_condition"
            case adherence, comment
        }
    }

    public func createReview(swapId: String, params: CreateReviewParams) async throws -> Review {
        return try await sendRequest(method: "POST", path: "/api/swaps/\(swapId)/reviews", body: params)
    }

    public func getUserReviews(userId: String) async throws -> [Review] {
        return try await sendRequest(method: "GET", path: "/api/users/\(userId)/reviews")
    }

    // MARK: - Notifications

    public func getNotifications(unreadOnly: Bool = false) async throws -> [Notification] {
        let q = [URLQueryItem(name: "unread", value: unreadOnly ? "true" : "false")]
        return try await sendRequest(method: "GET", path: "/api/notifications", queryItems: q)
    }

    // MARK: - Admin

    public func getAdminStats() async throws -> AdminStats {
        return try await sendRequest(method: "GET", path: "/api/admin/stats")
    }
}

// AnyCodable helper for loose key-value maps
public struct AnyCodable: Codable, Sendable {
    public let value: String

    public init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if let s = try? container.decode(String.self) {
            value = s
        } else if let b = try? container.decode(Bool.self) {
            value = "\(b)"
        } else if let i = try? container.decode(Int.self) {
            value = "\(i)"
        } else if let d = try? container.decode(Double.self) {
            value = "\(d)"
        } else {
            value = ""
        }
    }

    public func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        try container.encode(value)
    }
}
