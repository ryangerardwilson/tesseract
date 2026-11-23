import QtQuick
import QtQuick.Shapes

// SpaceSatellite is one window: a small bright electron on its shell with
// the window's class and title beside it. The chosen electron is the
// brightest, shows its title in full, and draws a short trail along its
// shell, and it leaves a streak as it rushes to the eye. A carried one glows
// brighter still, held in a tight halo. A window title is untrusted, so
// every text here is plain text.
Item {
    id: satellite

    required property SpaceCamera lens
    property var pose: ({
    })
    property string windowClass: ""
    property string windowTitle: ""
    // The halo's breath, which a carried electron's ring follows, and the
    // cool light of that ring.
    property real breath: 0
    property color cool: "white"
    readonly property real radius: pose.radius || 0
    readonly property real chosen: pose.chosen || 0
    readonly property real carried: pose.carried || 0
    readonly property real light: pose.light || 0
    readonly property bool rightward: (pose.side || 1) > 0
    readonly property real grow: pose.grow || 1
    readonly property real gap: (11 + 6 * chosen) * lens.unit * grow

    // Ribbon is a band of light along `trace`, points from the electron back
    // to where the band ends, `breadth` across at the electron. It narrows
    // to nothing and fades out toward its end.
    component Ribbon: Shape {
        id: ribbon

        property var trace: []
        property real breadth: 0
        property real light: 0
        readonly property point end: trace.length > 1 ? trace[trace.length - 1] : Qt.point(0, 0)

        // band outlines `share` of the ribbon's breadth: out along one side
        // and back along the other.
        function band(share) {
            const last = ribbon.trace.length - 1;
            if (last < 1)
                return [];

            const left = [];
            const right = [];
            for (let index = 0; index <= last; index++) {
                const before = ribbon.trace[Math.max(0, index - 1)];
                const after = ribbon.trace[Math.min(last, index + 1)];
                const dx = after.x - before.x;
                const dy = after.y - before.y;
                const span = Math.max(0.001, Math.sqrt(dx * dx + dy * dy));
                const half = share * ribbon.breadth / 2 * Math.pow(1 - index / last, 0.75);
                const at = ribbon.trace[index];
                left.push(Qt.point(at.x - dy / span * half, at.y + dx / span * half));
                right.unshift(Qt.point(at.x + dy / span * half, at.y - dx / span * half));
            }
            return left.concat(right);
        }

        preferredRendererType: Shape.CurveRenderer
        visible: light > 0.004 && trace.length > 1

        ShapePath {
            strokeColor: "transparent"
            fillRule: ShapePath.WindingFill

            fillGradient: LinearGradient {
                x1: 0
                y1: 0
                x2: ribbon.end.x
                y2: ribbon.end.y

                GradientStop {
                    position: 0
                    color: Qt.rgba(1, 1, 1, 0.2 * ribbon.light)
                }

                GradientStop {
                    position: 1
                    color: Qt.rgba(1, 1, 1, 0)
                }

            }

            PathPolyline {
                path: ribbon.band(1)
            }

        }

        ShapePath {
            strokeColor: "transparent"
            fillRule: ShapePath.WindingFill

            fillGradient: LinearGradient {
                x1: 0
                y1: 0
                x2: ribbon.end.x
                y2: ribbon.end.y

                GradientStop {
                    position: 0
                    color: Qt.rgba(1, 1, 1, 0.7 * ribbon.light)
                }

                GradientStop {
                    position: 1
                    color: Qt.rgba(1, 1, 1, 0)
                }

            }

            PathPolyline {
                path: ribbon.band(0.34)
            }

        }

    }

    x: pose.x || 0
    y: pose.y || 0
    opacity: pose.shown || 0
    visible: opacity > 0.004

    // The chosen electron's trail, back along its shell.
    Ribbon {
        trace: satellite.pose.trail || []
        breadth: satellite.radius * 2.3
        light: satellite.pose.trailLight || 0
    }

    // The streak the chosen electron leaves as it rushes to the eye.
    Ribbon {
        trace: satellite.pose.streak || []
        breadth: satellite.radius * 1.7
        light: satellite.pose.streakLight || 0
    }

    SpaceGlow {
        radius: satellite.radius * (3.6 + 2 * satellite.chosen - 1.2 * satellite.carried)
        strength: (0.12 + 0.2 * satellite.chosen + 0.3 * satellite.carried) * (0.5 + 0.5 * satellite.light)
    }

    // A carried electron is held in a tight halo.
    SpaceGlow {
        radius: satellite.radius * (1.9 + 0.15 * satellite.breath)
        strength: 0.45 * satellite.carried
        plateau: 0.36
        color: satellite.cool
    }

    SpaceSphere {
        radius: satellite.radius
        light: 0.5 + 0.5 * satellite.light
        glow: 1
    }

    SpaceGlow {
        radius: satellite.radius * 0.92
        strength: 0.7 + 0.3 * satellite.chosen
        plateau: 0.42
    }

    // The thin ring of the carried electron's halo, breathing with the
    // nucleus.
    Shape {
        preferredRendererType: Shape.CurveRenderer
        visible: satellite.carried > 0.01

        ShapePath {
            strokeColor: Qt.rgba(satellite.cool.r, satellite.cool.g, satellite.cool.b, 0.45 * satellite.carried)
            strokeWidth: satellite.lens.unit
            fillColor: "transparent"

            PathAngleArc {
                radiusX: satellite.radius * (1.3 + 0.06 * satellite.breath)
                radiusY: satellite.radius * (1.3 + 0.06 * satellite.breath)
                startAngle: 0
                sweepAngle: 360
            }

        }

    }

    Item {
        x: satellite.rightward ? satellite.gap : -satellite.gap - width
        y: -Math.round(8 * satellite.lens.unit)
        width: Math.round(190 * satellite.lens.unit)
        height: childrenRect.height
        opacity: 1 - satellite.lens.smooth(0.15, 0.6, satellite.chosen)
        visible: opacity > 0.01

        Text {
            width: parent.width
            horizontalAlignment: satellite.rightward ? Text.AlignLeft : Text.AlignRight
            text: satellite.windowClass
            textFormat: Text.PlainText
            elide: Text.ElideRight
            maximumLineCount: 1
            color: Qt.rgba(1, 1, 1, 0.38 + 0.5 * satellite.light)
            font.family: "JetBrainsMono Nerd Font"
            font.pixelSize: Math.round(10 * satellite.lens.unit)
            font.weight: Font.Bold
        }

        Text {
            y: Math.round(14 * satellite.lens.unit)
            width: parent.width
            horizontalAlignment: satellite.rightward ? Text.AlignLeft : Text.AlignRight
            text: satellite.windowTitle
            textFormat: Text.PlainText
            elide: Text.ElideRight
            maximumLineCount: 1
            color: Qt.rgba(1, 1, 1, 0.26 + 0.4 * satellite.light)
            font.family: "JetBrainsMono Nerd Font"
            font.pixelSize: Math.round(10 * satellite.lens.unit)
        }

    }

    Item {
        x: satellite.rightward ? satellite.gap : -satellite.gap - width
        y: -Math.round(9 * satellite.lens.unit * satellite.grow)
        width: Math.round(330 * satellite.lens.unit)
        height: childrenRect.height
        scale: satellite.grow
        transformOrigin: satellite.rightward ? Item.TopLeft : Item.TopRight
        opacity: satellite.lens.smooth(0.45, 0.9, satellite.chosen)
        visible: opacity > 0.01

        Text {
            width: parent.width
            horizontalAlignment: satellite.rightward ? Text.AlignLeft : Text.AlignRight
            text: satellite.windowClass
            textFormat: Text.PlainText
            elide: Text.ElideRight
            maximumLineCount: 1
            color: Qt.rgba(1, 1, 1, 0.62)
            font.family: "JetBrainsMono Nerd Font"
            font.pixelSize: Math.round(11 * satellite.lens.unit)
            font.weight: Font.Bold
            font.letterSpacing: 0.6 * satellite.lens.unit
        }

        Text {
            y: Math.round(18 * satellite.lens.unit)
            width: parent.width
            horizontalAlignment: satellite.rightward ? Text.AlignLeft : Text.AlignRight
            text: satellite.windowTitle
            textFormat: Text.PlainText
            wrapMode: Text.WrapAtWordBoundaryOrAnywhere
            elide: Text.ElideRight
            maximumLineCount: 2
            color: Qt.rgba(1, 1, 1, 0.96)
            font.family: "JetBrainsMono Nerd Font"
            font.pixelSize: Math.round(17 * satellite.lens.unit)
        }

    }

}
