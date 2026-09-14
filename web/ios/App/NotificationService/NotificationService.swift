import Intents
import UserNotifications

class NotificationService: UNNotificationServiceExtension {
    private let lock = NSLock()
    private var deliver: ((UNNotificationContent) -> Void)?
    private var arrived: UNNotificationContent?

    override func didReceive(
        _ request: UNNotificationRequest,
        withContentHandler contentHandler: @escaping (UNNotificationContent) -> Void
    ) {
        lock.lock()
        deliver = contentHandler
        arrived = request.content
        lock.unlock()

        guard let address = request.content.userInfo["pictureURL"] as? String,
              let pictureURL = URL(string: address),
              pictureURL.scheme == "https" else {
            finish(request.content)
            return
        }
        let senderName = request.content.userInfo["senderName"] as? String ?? request.content.title

        URLSession.shared.dataTask(with: pictureURL) { data, response, _ in
            guard let data, (response as? HTTPURLResponse)?.statusCode == 200 else {
                self.finish(request.content)
                return
            }
            self.finish(Self.fromSender(request.content, named: senderName, picture: INImage(imageData: data)))
        }.resume()
    }

    override func serviceExtensionTimeWillExpire() {
        lock.lock()
        let fallback = arrived
        lock.unlock()
        if let fallback {
            finish(fallback)
        }
    }

    private func finish(_ content: UNNotificationContent) {
        lock.lock()
        let handler = deliver
        deliver = nil
        lock.unlock()
        handler?(content)
    }

    private static func fromSender(
        _ content: UNNotificationContent,
        named senderName: String,
        picture: INImage
    ) -> UNNotificationContent {
        let sender = INPerson(
            personHandle: INPersonHandle(value: senderName, type: .unknown),
            nameComponents: nil,
            displayName: senderName,
            image: picture,
            contactIdentifier: nil,
            customIdentifier: nil
        )
        let intent = INSendMessageIntent(
            recipients: nil,
            outgoingMessageType: .outgoingMessageText,
            content: content.body,
            speakableGroupName: nil,
            conversationIdentifier: content.threadIdentifier,
            serviceName: nil,
            sender: sender,
            attachments: nil
        )
        intent.setImage(picture, forParameterNamed: \.sender)
        let interaction = INInteraction(intent: intent, response: nil)
        interaction.direction = .incoming
        interaction.donate(completion: nil)
        return (try? content.updating(from: intent)) ?? content
    }
}
