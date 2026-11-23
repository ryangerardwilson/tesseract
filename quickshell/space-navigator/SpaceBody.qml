import QtQuick
import QtQuick.Shapes

// SpaceBody is one workspace as a glass sphere with its number on it: the
// nucleus at the centre, large and glowing, or a small body on the orbit,
// dimmer the further round the orbit it turns. The nucleus wears a corona
// that breathes with the halo and a soft filament that sways with the chosen
// electron, and both bloom as the system opens or settles.
Item {
    id: body

    required property var space
    required property var pose
    required property bool active
    // The nucleus's light, from the stage: the halo's breath, how far the
    // nucleus blooms, the chosen electron's swing, and the tints of its core
    // and its rim.
    property real breath: 0
    property real bloom: 0
    property real sway: 0
    property color warm: "white"
    property color cool: "white"
    // How much of the nucleus this body is. Its corona and filament fade as
    // it shrinks back onto the orbit.
    readonly property real core: pose.hold * pose.hold * pose.seen

    // Wisp is one strand of the filament on a 200 by 200 face that scales
    // with the body: a low arch that leaves the rim, rises `rise` off it, and
    // comes back to it `span` radians round, thickest and brightest at its
    // crown, so it reads as a strand of light and not as a ring.
    component Wisp: Shape {
        id: wisp

        property real span: 2
        property real rise: 14
        property color color: "white"

        // strand outlines the arch `thickness` across at its crown,
        // narrowing to nothing where it meets the rim.
        function strand(thickness) {
            const outer = [];
            const inner = [];
            const steps = 24;
            for (let i = 0; i <= steps; i++) {
                const along = i / steps;
                const arch = Math.sin(Math.PI * along);
                const angle = wisp.span * (along - 0.5);
                const middle = 100 + wisp.rise * arch;
                const half = thickness / 2 * Math.pow(arch, 0.6);
                outer.push(Qt.point(100 + (middle + half) * Math.sin(angle), 100 - (middle + half) * Math.cos(angle)));
                inner.unshift(Qt.point(100 + (middle - half) * Math.sin(angle), 100 - (middle - half) * Math.cos(angle)));
            }
            return outer.concat(inner);
        }

        x: -100
        y: -100
        width: 200
        height: 200
        preferredRendererType: Shape.CurveRenderer
        visible: opacity > 0.004

        ShapePath {
            strokeColor: "transparent"
            fillColor: Qt.rgba(wisp.color.r, wisp.color.g, wisp.color.b, 0.07)

            PathPolyline {
                path: wisp.strand(13)
            }

        }

        ShapePath {
            strokeColor: "transparent"
            fillColor: Qt.rgba(wisp.color.r, wisp.color.g, wisp.color.b, 0.1)

            PathPolyline {
                path: wisp.strand(6)
            }

        }

        ShapePath {
            strokeColor: "transparent"
            fillColor: Qt.rgba(wisp.color.r, wisp.color.g, wisp.color.b, 0.32)

            PathPolyline {
                path: wisp.strand(1.8)
            }

        }

    }

    x: pose.x
    y: pose.y
    z: -pose.depth

    // The corona: light that hugs the nucleus's rim, swelling and brightening
    // with each breath of the halo and flaring as the nucleus blooms.
    SpaceGlow {
        radius: body.pose.radius * (1.55 + 0.14 * body.breath + 0.4 * body.bloom)
        strength: (0.1 + 0.07 * body.breath + 0.16 * body.bloom) * body.core
        plateau: 0.6
        color: body.cool
    }

    // The filament: two soft strands arching off the rim that sway the other
    // way to each other as the chosen electron swings, and brighten in turn
    // as the halo breathes.
    Wisp {
        scale: body.pose.radius / 100
        rotation: -62 + 24 * body.sway
        opacity: body.core * (0.5 + 0.4 * body.breath + 0.4 * body.bloom)
        span: 2.1
        rise: body.core > 0.004 ? 13 + 4 * body.breath : 13
        color: body.cool
    }

    Wisp {
        scale: body.pose.radius / 100
        rotation: 128 - 18 * body.sway + 10 * body.breath
        opacity: body.core * (0.35 + 0.4 * (1 - body.breath) + 0.4 * body.bloom)
        span: 1.5
        rise: body.core > 0.004 ? 10 + 4 * (1 - body.breath) : 10
        color: body.cool
    }

    SpaceSphere {
        opacity: body.pose.seen
        visible: opacity > 0.004
        radius: body.pose.radius
        light: body.pose.light
        glow: body.pose.hold * (1 + 0.45 * body.bloom)
        core: body.warm
        rim: body.cool

        Text {
            anchors.centerIn: parent
            width: 172
            horizontalAlignment: Text.AlignHCenter
            text: String(body.space.name || body.space.id)
            textFormat: Text.PlainText
            renderType: Text.CurveRendering
            elide: Text.ElideRight
            maximumLineCount: 1
            color: Qt.rgba(1, 1, 1, 0.5 + 0.5 * body.pose.light)
            font.family: "JetBrainsMono Nerd Font"
            font.pixelSize: 78
            font.weight: Font.Bold
        }

        Rectangle {
            readonly property real size: Math.max(13, 5 * 100 / Math.max(1, body.pose.radius))

            visible: body.active
            x: 176 - size / 2
            y: 24 - size / 2
            width: size
            height: size
            radius: size / 2
            antialiasing: true
            color: "#ffffff"
            opacity: 0.45 + 0.55 * body.pose.light
        }

    }

}
