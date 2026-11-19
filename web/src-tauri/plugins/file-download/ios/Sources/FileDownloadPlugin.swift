import Foundation
import Tauri
import WebKit

private let unnamedMediaType = "application/octet-stream"
private let progressInterval: TimeInterval = 0.1

class DownloadRequest: Decodable {
    let address: String
    let destination: String
    let onProgress: Channel
}

struct DownloadProgress: Encodable {
    let receivedBytes: Int64
    let totalBytes: Int64
}

struct Downloaded: Encodable {
    let mediaType: String
}

struct DownloadFailure: Error {
    let message: String
}

final class Delivery {
    let invoke: Invoke
    let destination: URL
    let onProgress: Channel
    var lastReportedAt = Date.distantPast
    var outcome: Result<String, DownloadFailure>?

    init(invoke: Invoke, destination: URL, onProgress: Channel) {
        self.invoke = invoke
        self.destination = destination
        self.onProgress = onProgress
    }
}

final class DownloadDelegate: NSObject, URLSessionDownloadDelegate {
    let queue: OperationQueue = {
        let serial = OperationQueue()
        serial.maxConcurrentOperationCount = 1
        return serial
    }()
    private var deliveries: [Int: Delivery] = [:]

    func expect(_ delivery: Delivery, from task: URLSessionDownloadTask) {
        queue.addOperation {
            self.deliveries[task.taskIdentifier] = delivery
            task.resume()
        }
    }

    func cancelUnexpected(_ tasks: [URLSessionTask]) {
        queue.addOperation {
            tasks.filter { self.deliveries[$0.taskIdentifier] == nil }.forEach { $0.cancel() }
        }
    }

    func urlSession(
        _ session: URLSession, downloadTask: URLSessionDownloadTask, didWriteData bytesWritten: Int64,
        totalBytesWritten: Int64, totalBytesExpectedToWrite: Int64
    ) {
        guard let delivery = deliveries[downloadTask.taskIdentifier] else { return }
        let now = Date()
        let isComplete = totalBytesWritten == totalBytesExpectedToWrite
        guard isComplete || now.timeIntervalSince(delivery.lastReportedAt) >= progressInterval else { return }
        delivery.lastReportedAt = now
        do {
            try delivery.onProgress.send(DownloadProgress(receivedBytes: totalBytesWritten, totalBytes: max(0, totalBytesExpectedToWrite)))
        } catch {
            Logger.error("the progress of \(delivery.destination.lastPathComponent) was not reported: \(error)")
        }
    }

    func urlSession(_ session: URLSession, downloadTask: URLSessionDownloadTask, didFinishDownloadingTo location: URL) {
        guard let delivery = deliveries[downloadTask.taskIdentifier] else { return }
        delivery.outcome = Self.kept(location, from: downloadTask, at: delivery.destination)
    }

    func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError error: Error?) {
        guard let delivery = deliveries.removeValue(forKey: task.taskIdentifier) else { return }
        if let error = error {
            delivery.invoke.reject("the download of \(delivery.destination.lastPathComponent) stopped: \(error.localizedDescription)")
            return
        }
        switch delivery.outcome {
        case .success(let mediaType):
            delivery.invoke.resolve(Downloaded(mediaType: mediaType))
        case .failure(let failure):
            delivery.invoke.reject(failure.message)
        case nil:
            delivery.invoke.reject("the download of \(delivery.destination.lastPathComponent) ended without a file")
        }
    }

    private static func kept(_ location: URL, from task: URLSessionDownloadTask, at destination: URL) -> Result<String, DownloadFailure> {
        if let answer = task.response as? HTTPURLResponse, !(200..<300).contains(answer.statusCode) {
            return .failure(DownloadFailure(message: "\(destination.lastPathComponent) did not arrive: HTTP \(answer.statusCode)"))
        }
        do {
            if FileManager.default.fileExists(atPath: destination.path) {
                try FileManager.default.removeItem(at: destination)
            }
            try FileManager.default.moveItem(at: location, to: destination)
        } catch {
            return .failure(DownloadFailure(message: "the download could not be moved to \(destination.path): \(error.localizedDescription)"))
        }
        return .success(task.response?.mimeType ?? unnamedMediaType)
    }
}

class FileDownloadPlugin: Plugin {
    private let delegate = DownloadDelegate()
    private lazy var session: URLSession = {
        let identifier = "\(Bundle.main.bundleIdentifier ?? "app.intern.kim").file-download"
        let configuration = URLSessionConfiguration.background(withIdentifier: identifier)
        configuration.sessionSendsLaunchEvents = false
        return URLSession(configuration: configuration, delegate: delegate, delegateQueue: delegate.queue)
    }()

    @objc public override func load(webview: WKWebView) {
        session.getAllTasks { leftovers in
            self.delegate.cancelUnexpected(leftovers)
        }
    }

    @objc public func download(_ invoke: Invoke) throws {
        let request = try invoke.parseArgs(DownloadRequest.self)
        guard let address = URL(string: request.address) else {
            invoke.reject("\(request.address) is not an address to download from")
            return
        }
        let delivery = Delivery(invoke: invoke, destination: URL(fileURLWithPath: request.destination), onProgress: request.onProgress)
        delegate.expect(delivery, from: session.downloadTask(with: address))
    }
}

@_cdecl("init_plugin_file_download")
public func initPlugin() -> Plugin {
    return FileDownloadPlugin()
}
