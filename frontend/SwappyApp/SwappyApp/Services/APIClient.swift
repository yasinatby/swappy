import Foundation
import Combine

@MainActor
public class APIClient: ObservableObject {
    public static let shared = APIClient()

    @Published public var backendURL: String {
        didSet {
            UserDefaults.standard.set(backendURL, forKey: "swappy_backend_url")
        }
    }
    @Published public var currentUser: User?
    @Published public var token: String?
    @Published public var isAuthenticated: Bool = false
    @Published public var errorMessage: String?

    private let session: URLSession
    private let decoder = JSONDecoder()
    private let encoder = JSONEncoder()

    public init() {
        let savedURL = UserDefaults.standard.string(forKey: "swappy_backend_url") ?? "http://localhost:8080"
        self.backendURL = savedURL
        self.session = .shared
    }

    private var baseURL: URL {
        URL(string: backendURL) ?? URL(string: "http://localhost:8080")!
    }

    // MARK: - Generic HTTP

    private func request<T: Decodable>(
        method: String,
        path: String,
        query: [URLQueryItem]? = nil,
        body: (any Encodable)? = nil
    ) async throws -> T {
        var components = URLComponents(url: baseURL.appendingPathComponent(path), resolvingAgainstBaseURL: true)
        if let query = query, !query.isEmpty {
            components?.queryItems = query
        }

        guard let url = components?.url else {
            throw URLError(.badURL)
        }

        var req = URLRequest(url: url)
        req.httpMethod = method
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")

        if let token = self.token {
            req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }

        if let body = body {
            req.httpBody = try encoder.encode(body)
        }

        let (data, response) = try await session.data(for: req)
        guard let httpResponse = response as? HTTPURLResponse else {
            throw URLError(.badServerResponse)
        }

        if httpResponse.statusCode < 200 || httpResponse.statusCode >= 300 {
            if let errObj = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
               let msg = errObj["error"] as? String {
                throw NSError(domain: "SwappyError", code: httpResponse.statusCode, userInfo: [NSLocalizedDescriptionKey: msg])
            }
            let rawStr = String(data: data, encoding: .utf8) ?? "HTTP \(httpResponse.statusCode)"
            throw NSError(domain: "SwappyError", code: httpResponse.statusCode, userInfo: [NSLocalizedDescriptionKey: rawStr])
        }

        return try decoder.decode(T.self, from: data)
    }

    // MARK: - Auth & Demo

    public func loginDemo(as user: String) async throws {
        // Automatically register or login demo users
        let username = user.lowercased()
        let email = "\(username)@swappy.local"
        let password = "password123"

        struct AuthReq: Encodable {
            let username_or_email: String
            let password: String
        }

        struct RegReq: Encodable {
            let username: String
            let email: String
            let password: String
            let first_name: String
            let city: String
        }

        do {
            let res: AuthResponse = try await request(
                method: "POST",
                path: "/api/auth/login",
                body: AuthReq(username_or_email: username, password: password)
            )
            self.token = res.token
            self.currentUser = res.user
            self.isAuthenticated = true
        } catch {
            // If user does not exist yet, register them
            let regRes: AuthResponse = try await request(
                method: "POST",
                path: "/api/auth/register",
                body: RegReq(
                    username: username,
                    email: email,
                    password: password,
                    first_name: username.capitalized,
                    city: username == "max" ? "Heilbronn" : "Stuttgart"
                )
            )
            self.token = regRes.token
            self.currentUser = regRes.user
            self.isAuthenticated = true
        }
    }

    public func logout() {
        self.token = nil
        self.currentUser = nil
        self.isAuthenticated = false
    }

    // MARK: - Items

    public func fetchHomeFeed() async throws -> HomeFeed {
        return try await request(method: "GET", path: "/api/feed/home")
    }

    public func fetchItems(
        query: String? = nil,
        category: String? = nil,
        tradeInterest: String? = nil
    ) async throws -> [Item] {
        var q: [URLQueryItem] = []
        if let query = query, !query.isEmpty { q.append(URLQueryItem(name: "q", value: query)) }
        if let category = category, !category.isEmpty { q.append(URLQueryItem(name: "category", value: category)) }
        if let tradeInterest = tradeInterest, !tradeInterest.isEmpty { q.append(URLQueryItem(name: "trade_interest", value: tradeInterest)) }
        return try await request(method: "GET", path: "/api/items", query: q)
    }

    public func fetchItem(id: String) async throws -> Item {
        return try await request(method: "GET", path: "/api/items/\(id)")
    }

    public struct CreateItemBody: Encodable {
        public let title: String
        public let description: String
        public let category: String
        public let condition: String
        public let estimated_value: Double
        public let trade_interests: [String]
        public let images: [String]
        public let city: String
    }

    public func createItem(
        title: String,
        description: String,
        category: String,
        condition: String,
        value: Double,
        tradeInterests: [String],
        city: String
    ) async throws -> Item {
        let body = CreateItemBody(
            title: title,
            description: description,
            category: category,
            condition: condition,
            estimated_value: value,
            trade_interests: tradeInterests,
            images: [],
            city: city
        )
        return try await request(method: "POST", path: "/api/items", body: body)
    }

    public func toggleFavorite(itemId: String) async throws {
        struct FavRes: Decodable { let is_favorited: Bool }
        let _: FavRes = try await request(method: "POST", path: "/api/items/\(itemId)/favorite")
    }

    public func fetchFavorites() async throws -> [Item] {
        return try await request(method: "GET", path: "/api/favorites")
    }

    // MARK: - Swaps

    public func fetchMatches() async throws -> [SwapMatch] {
        return try await request(method: "GET", path: "/api/swaps/matches")
    }

    public func fetchSwaps(status: String? = nil) async throws -> [Swap] {
        var q: [URLQueryItem] = []
        if let s = status { q.append(URLQueryItem(name: "status", value: s)) }
        return try await request(method: "GET", path: "/api/swaps", query: q)
    }

    public func fetchSwap(id: String) async throws -> Swap {
        return try await request(method: "GET", path: "/api/swaps/\(id)")
    }

    public struct ProposeBody: Encodable {
        public let target_item_id: String
        public let offered_item_ids: [String]
        public let cash_adjustment: Double
        public let message: String
        public let handover_method: String
    }

    public func proposeSwap(
        targetItemId: String,
        offeredItemIds: [String],
        cashAdjustment: Double,
        message: String
    ) async throws -> Swap {
        let body = ProposeBody(
            target_item_id: targetItemId,
            offered_item_ids: offeredItemIds,
            cash_adjustment: cashAdjustment,
            message: message,
            handover_method: "personal"
        )
        return try await request(method: "POST", path: "/api/swaps", body: body)
    }

    public struct CounterBody: Encodable {
        public let offered_item_ids: [String]
        public let cash_adjustment: Double
        public let message: String
    }

    public func counterSwap(
        swapId: String,
        offeredItemIds: [String],
        cashAdjustment: Double,
        message: String
    ) async throws -> Swap {
        let body = CounterBody(
            offered_item_ids: offeredItemIds,
            cash_adjustment: cashAdjustment,
            message: message
        )
        return try await request(method: "POST", path: "/api/swaps/\(swapId)/counter", body: body)
    }

    public func acceptSwap(swapId: String) async throws -> Swap {
        return try await request(method: "POST", path: "/api/swaps/\(swapId)/accept")
    }

    public func rejectSwap(swapId: String) async throws -> Swap {
        return try await request(method: "POST", path: "/api/swaps/\(swapId)/reject")
    }

    public func completeSwap(swapId: String) async throws -> Swap {
        return try await request(method: "POST", path: "/api/swaps/\(swapId)/complete")
    }

    // MARK: - Chat

    public func fetchMessages(swapId: String) async throws -> [Message] {
        return try await request(method: "GET", path: "/api/swaps/\(swapId)/messages")
    }

    public func sendMessage(swapId: String, text: String) async throws -> Message {
        struct MsgBody: Encodable { let text: String }
        return try await request(method: "POST", path: "/api/swaps/\(swapId)/messages", body: MsgBody(text: text))
    }

    // MARK: - Review

    public struct ReviewBody: Encodable {
        public let rating: Int
        public let reliability: Int
        public let communication: Int
        public let item_condition: Int
        public let adherence: Int
        public let comment: String
    }

    public func submitReview(
        swapId: String,
        rating: Int,
        reliability: Int = 5,
        communication: Int = 5,
        itemCondition: Int = 5,
        adherence: Int = 5,
        comment: String
    ) async throws -> Review {
        let body = ReviewBody(
            rating: rating,
            reliability: reliability,
            communication: communication,
            item_condition: itemCondition,
            adherence: adherence,
            comment: comment
        )
        return try await request(method: "POST", path: "/api/swaps/\(swapId)/reviews", body: body)
    }

    // MARK: - Profile

    public func fetchUserProfile(id: String) async throws -> UserPublicProfile {
        return try await request(method: "GET", path: "/api/users/\(id)")
    }
}
