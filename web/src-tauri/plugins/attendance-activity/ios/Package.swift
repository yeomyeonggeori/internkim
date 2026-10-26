// swift-tools-version:5.5

import PackageDescription

let package = Package(
    name: "tauri-plugin-attendance-activity",
    platforms: [
        .macOS(.v10_13),
        .iOS(.v15),
    ],
    products: [
        .library(
            name: "tauri-plugin-attendance-activity",
            type: .static,
            targets: ["tauri-plugin-attendance-activity"]),
    ],
    dependencies: [
        .package(name: "Tauri", path: "../.tauri/tauri-api")
    ],
    targets: [
        .target(
            name: "tauri-plugin-attendance-activity",
            dependencies: [
                .byName(name: "Tauri")
            ],
            path: "Sources")
    ]
)
