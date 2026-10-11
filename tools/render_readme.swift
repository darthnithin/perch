// Render a captured VT screen with the approved profile's actual macOS font.
// Offline CoreText drawing only; this does not access or control Terminal.app.
import AppKit
import CoreText

struct Cell: Decodable {
    let text: String
    let foreground: String
    let background: String
    let bold: Bool
    let underline: Bool
}
struct Screen: Decodable {
    let columns: Int
    let rows: Int
    let cells: [[Cell]]
}
let args = CommandLine.arguments
precondition(args.count == 5, "render_readme screens.json profile.terminal output-directory scale")
let screens = try JSONDecoder().decode([Screen].self, from: Data(contentsOf: URL(fileURLWithPath: args[1])))
let profile = try PropertyListSerialization.propertyList(from: Data(contentsOf: URL(fileURLWithPath: args[2])), format: nil) as! [String: Any]
let profileFont = try NSKeyedUnarchiver.unarchivedObject(ofClass: NSFont.self, from: profile["Font"] as! Data)!
func loadFont(_ filename: String) -> NSFont {
    let url = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent("Library/Fonts/" + filename)
    guard let provider = CGDataProvider(url: url as CFURL), let graphicsFont = CGFont(provider) else {
        fatalError("Install the approved font at " + url.path)
    }
    CTFontManagerRegisterFontsForURL(url as CFURL, .process, nil)
    return CTFontCreateWithGraphicsFont(graphicsFont, profileFont.pointSize, nil, nil) as NSFont
}
let font = loadFont("IoskeleyMono-Regular.ttf")
let bold = loadFont("IoskeleyMono-Bold.ttf")
let spacing = CGFloat(profile["FontHeightSpacing"] as! Double)
let widthSpacing = CGFloat(profile["FontWidthSpacing"] as! Double)
let scale = CGFloat(Double(args[4])!)
let cellWidth = ("M" as NSString).size(withAttributes: [.font: font]).width * widthSpacing
let naturalHeight = NSLayoutManager().defaultLineHeight(for: font)
let cellHeight = naturalHeight * spacing
let padding: CGFloat = 16
func render(_ screen: Screen, to output: URL) throws {
let width = ceil(CGFloat(screen.columns) * cellWidth + 2 * padding)
let height = ceil(CGFloat(screen.rows) * cellHeight + 2 * padding)
let context = CGContext(data: nil, width: Int(width * scale), height: Int(height * scale), bitsPerComponent: 8, bytesPerRow: 0, space: CGColorSpace(name: CGColorSpace.sRGB)!, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
context.scaleBy(x: scale, y: scale)
func color(_ hex: String) -> CGColor {
    let value = UInt32(hex, radix: 16)!
    return CGColor(red: CGFloat((value >> 16) & 255) / 255, green: CGFloat((value >> 8) & 255) / 255, blue: CGFloat(value & 255) / 255, alpha: 1)
}
context.setFillColor(color("1c1b1a"))
context.fill(CGRect(x: 0, y: 0, width: width, height: height))
context.setShouldAntialias(false)
// Paint all cell backgrounds first so the next row cannot erase glyph overhangs.
for (row, cells) in screen.cells.enumerated() {
    for (column, cell) in cells.enumerated() {
        context.setFillColor(color(cell.background))
        context.fill(CGRect(x: padding + CGFloat(column) * cellWidth, y: height - padding - CGFloat(row + 1) * cellHeight, width: cellWidth, height: cellHeight))
    }
}
context.setShouldAntialias(true)
for (row, cells) in screen.cells.enumerated() {
    for (column, cell) in cells.enumerated() {
        let face = cell.bold ? bold : font
        let x = padding + CGFloat(column) * cellWidth
        let bottom = height - padding - CGFloat(row + 1) * cellHeight
        // Center the font's ascent/descent in the profile's cell height.
        let baseline = bottom + (cellHeight - face.ascender + face.descender) / CGFloat(2) - face.descender
        if cell.text != " " && !cell.text.isEmpty {
            let attributes: [NSAttributedString.Key: Any] = [.font: face, .foregroundColor: NSColor(cgColor: color(cell.foreground))!]
            let line = CTLineCreateWithAttributedString(NSAttributedString(string: cell.text, attributes: attributes))
            context.textMatrix = .identity
            context.textPosition = CGPoint(x: x, y: baseline)
            CTLineDraw(line, context)
        }
        if cell.underline {
            context.setFillColor(color(cell.foreground))
            context.fill(CGRect(x: x, y: baseline + face.underlinePosition, width: cellWidth, height: max(1 / scale, face.underlineThickness)))
        }
    }
}
let bitmap = NSBitmapImageRep(cgImage: context.makeImage()!)
try bitmap.representation(using: .png, properties: [:])!.write(to: output)
}
let directory = URL(fileURLWithPath: args[3])
try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
for (index, screen) in screens.enumerated() {
    try render(screen, to: directory.appendingPathComponent(String(format: "frame-%03d.png", index)))
}
print("\(font.fontName) \(font.pointSize) pt; natural height \(naturalHeight), spacing \(spacing), cell \(cellWidth) × \(cellHeight), \(scale)× raster; \(screens.count) frames")
