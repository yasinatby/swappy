import PhotosUI
import SwiftUI

@MainActor
@Observable
final class ItemEditorModel {
    static let maxImages = 6

    let api: SwappyAPI
    private(set) var item: Item?

    var title = ""
    var description = ""
    var wantedText = ""
    var category = "sonstiges"
    var condition: ItemCondition = .good

    /// Already uploaded images (editing an existing item).
    private(set) var existingImages: [ItemImage] = []
    /// Compressed photos not uploaded yet (new item, or added while editing).
    private(set) var pendingImages: [Data] = []

    private(set) var isSaving = false
    var error: String?

    init(api: SwappyAPI, item: Item?) {
        self.api = api
        self.item = item
        if let item {
            title = item.title
            description = item.description ?? ""
            wantedText = item.wantedText ?? ""
            category = item.category
            condition = item.condition
            existingImages = item.images ?? []
        }
    }

    var isNew: Bool { item == nil }
    var imageCount: Int { existingImages.count + pendingImages.count }
    var canSave: Bool { !title.trimmingCharacters(in: .whitespaces).isEmpty && !isSaving }

    private var input: ItemInput {
        func optional(_ s: String) -> String? {
            let t = s.trimmingCharacters(in: .whitespacesAndNewlines)
            return t.isEmpty ? nil : t
        }
        return ItemInput(title: title.trimmingCharacters(in: .whitespaces), description: optional(description),
                         category: category, condition: condition, wantedText: optional(wantedText))
    }

    func addPhotos(_ selection: [PhotosPickerItem]) async {
        for picked in selection.prefix(Self.maxImages - imageCount) {
            guard let data = try? await picked.loadTransferable(type: Data.self),
                  let jpeg = ImageCompressor.jpeg(from: data) else {
                error = "Ein Foto konnte nicht geladen werden."
                continue
            }
            pendingImages.append(jpeg)
        }
    }

    func removePending(at index: Int) {
        pendingImages.remove(at: index)
    }

    func removeExisting(_ image: ItemImage) async {
        guard let item else { return }
        do {
            try await api.deleteImage(itemId: item.id, imageId: image.id)
            existingImages.removeAll { $0.id == image.id }
        } catch {
            self.error = errorMessage(for: error)
        }
    }

    /// Creates or updates the item, then uploads pending photos.
    func save(publish: Bool) async -> Bool {
        isSaving = true
        defer { isSaving = false }
        do {
            var saved: Item
            if let item {
                saved = try await api.updateItem(item.id, input)
                if publish && saved.status == .draft {
                    saved = try await api.publishItem(item.id)
                }
            } else {
                saved = try await api.createItem(input, publish: publish)
            }
            item = saved
            while let jpeg = pendingImages.first {
                let image = try await api.uploadImage(itemId: saved.id, jpeg: jpeg)
                existingImages.append(image)
                pendingImages.removeFirst()
            }
            return true
        } catch {
            self.error = errorMessage(for: error)
            return false
        }
    }
}

struct ItemEditorView: View {
    @Environment(\.dismiss) private var dismiss
    @State private var model: ItemEditorModel
    @State private var pickerSelection: [PhotosPickerItem] = []
    private let onSaved: (() -> Void)?

    init(api: SwappyAPI, item: Item? = nil, onSaved: (() -> Void)? = nil) {
        _model = State(initialValue: ItemEditorModel(api: api, item: item))
        self.onSaved = onSaved
    }

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    photos
                } header: {
                    Text("Fotos (\(model.imageCount)/\(ItemEditorModel.maxImages))")
                } footer: {
                    Text("Das erste Foto ist das Titelbild.")
                }

                Section("Was gibst du her?") {
                    TextField("Titel", text: $model.title)
                        .accessibilityIdentifier("itemTitleField")
                    Picker("Kategorie", selection: $model.category) {
                        ForEach(Category.all) { Label($0.label, systemImage: $0.symbol).tag($0.id) }
                    }
                    Picker("Zustand", selection: $model.condition) {
                        ForEach(ItemCondition.allCases) { Text($0.label).tag($0) }
                    }
                    .pickerStyle(.segmented)
                    TextField("Beschreibung", text: $model.description, axis: .vertical)
                        .lineLimit(3 ... 8)
                }

                Section {
                    TextField("z. B. Fahrrad, Bücher oder Werkzeug", text: $model.wantedText, axis: .vertical)
                        .lineLimit(1 ... 4)
                } header: {
                    Text("Das suche ich dafür")
                } footer: {
                    Text("Optional. Hilft anderen, ein passendes Angebot zu machen.")
                }
            }
            .navigationTitle(model.isNew ? "Neuer Gegenstand" : "Bearbeiten")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Abbrechen") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    if model.isSaving {
                        ProgressView()
                    } else if model.item?.status == .active {
                        Button("Sichern") { Task { await save(publish: false) } }
                            .disabled(!model.canSave)
                    } else {
                        Menu("Sichern") {
                            Button("Veröffentlichen", systemImage: "paperplane") { Task { await save(publish: true) } }
                            Button("Als Entwurf sichern", systemImage: "doc") { Task { await save(publish: false) } }
                        }
                        .disabled(!model.canSave)
                        .accessibilityIdentifier("saveItemMenu")
                    }
                }
            }
            .interactiveDismissDisabled(model.isSaving)
            .errorAlert($model.error)
            .onChange(of: pickerSelection) { _, selection in
                guard !selection.isEmpty else { return }
                Task {
                    await model.addPhotos(selection)
                    pickerSelection = []
                }
            }
        }
    }

    private func save(publish: Bool) async {
        if await model.save(publish: publish) {
            onSaved?()
            dismiss()
        }
    }

    private var photos: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 10) {
                ForEach(model.existingImages) { image in
                    thumbnail(ItemThumbnail(url: model.api.imageURL(image.path), cornerRadius: 10)) {
                        Task { await model.removeExisting(image) }
                    }
                }
                ForEach(Array(model.pendingImages.enumerated()), id: \.offset) { index, data in
                    thumbnail(
                        Image(uiImage: UIImage(data: data) ?? UIImage())
                            .resizable()
                            .scaledToFill()
                            .clipShape(RoundedRectangle(cornerRadius: 10))
                    ) {
                        model.removePending(at: index)
                    }
                }
                if model.imageCount < ItemEditorModel.maxImages {
                    PhotosPicker(selection: $pickerSelection,
                                 maxSelectionCount: ItemEditorModel.maxImages - model.imageCount,
                                 matching: .images) {
                        VStack(spacing: 4) {
                            Image(systemName: "camera.fill").font(.title2)
                            Text("Hinzufügen").font(.caption)
                        }
                        .frame(width: 84, height: 84)
                        .background(Color.swappyAccent.opacity(0.12), in: RoundedRectangle(cornerRadius: 10))
                        .foregroundStyle(Color.swappyAccent)
                    }
                }
            }
            .padding(.vertical, 4)
        }
    }

    private func thumbnail(_ content: some View, remove: @escaping () -> Void) -> some View {
        content
            .frame(width: 84, height: 84)
            .clipped()
            .overlay(alignment: .topTrailing) {
                Button(action: remove) {
                    Image(systemName: "xmark.circle.fill")
                        .symbolRenderingMode(.palette)
                        .foregroundStyle(.white, .black.opacity(0.6))
                        .font(.title3)
                }
                .buttonStyle(.plain)
                .padding(4)
            }
    }
}
