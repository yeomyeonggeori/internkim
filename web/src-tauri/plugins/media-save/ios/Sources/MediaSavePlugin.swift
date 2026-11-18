import Photos
import Tauri

class GalleryItem: Decodable {
    let path: String
    let mediaType: String
}

class MediaSavePlugin: Plugin {
    @objc public func saveIntoGallery(_ invoke: Invoke) throws {
        let item = try invoke.parseArgs(GalleryItem.self)
        PHPhotoLibrary.requestAuthorization(for: .addOnly) { status in
            guard status == .authorized else {
                invoke.reject("the app may not add to the photo library (authorization status \(status.rawValue))")
                return
            }
            Self.add(item, invoke)
        }
    }

    private static func add(_ item: GalleryItem, _ invoke: Invoke) {
        let fileURL = URL(fileURLWithPath: item.path)
        let resourceType: PHAssetResourceType = item.mediaType.hasPrefix("video/") ? .video : .photo
        PHPhotoLibrary.shared().performChanges({
            let options = PHAssetResourceCreationOptions()
            options.originalFilename = fileURL.lastPathComponent
            PHAssetCreationRequest.forAsset().addResource(with: resourceType, fileURL: fileURL, options: options)
        }) { isSaved, error in
            if let error = error {
                invoke.reject("the photo library refused \(fileURL.lastPathComponent): \(error.localizedDescription)")
                return
            }
            guard isSaved else {
                invoke.reject("the photo library did not keep \(fileURL.lastPathComponent)")
                return
            }
            invoke.resolve()
        }
    }
}

@_cdecl("init_plugin_media_save")
public func initPlugin() -> Plugin {
    return MediaSavePlugin()
}
