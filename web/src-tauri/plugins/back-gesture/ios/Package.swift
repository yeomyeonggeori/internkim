// swift-tools-version:5.5

import PackageDescription

let package = Package(
    name: "tauri-plugin-back-gesture",
    platforms: [
        .macOS(.v10_13),
        .iOS(.v15),
    ],
    products: [
        .library(
            name: "tauri-plugin-back-gesture",
            type: .static,
            targets: ["tauri-plugin-back-gesture"]),
    ],
    dependencies: [
        .package(name: "Tauri", path: "../.tauri/tauri-api")
    ],
    targets: [
        .target(
            name: "tauri-plugin-back-gesture",
            dependencies: [
                .byName(name: "Tauri")
            ],
            path: "Sources")
    ]
)
