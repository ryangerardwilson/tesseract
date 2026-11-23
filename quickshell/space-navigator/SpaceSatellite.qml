import QtQuick

// SpaceSatellite is one window: a small bright electron on its shell with
// the window's class and title beside it. The chosen electron is the
// brightest and shows its title in full, and a carried one glows brighter
// still. A window title is untrusted, so every text here is plain text.
Item {
    id: satellite

    required property SpaceCamera lens
    property var pose: ({
    })
    property string windowClass: ""
    property string windowTitle: ""
    readonly property real chosen: pose.chosen || 0
    readonly property real carried: pose.carried || 0
    readonly property real light: pose.light || 0
    readonly property bool rightward: (pose.side || 1) > 0
    readonly property real grow: pose.grow || 1
    readonly property real gap: (11 + 6 * chosen) * lens.unit * grow

    x: pose.x || 0
    y: pose.y || 0
    opacity: pose.shown || 0
    visible: opacity > 0.004

    SpaceGlow {
        radius: (satellite.pose.radius || 0) * (3.6 + 2 * satellite.chosen + 2 * satellite.carried)
        strength: (0.12 + 0.2 * satellite.chosen + 0.3 * satellite.carried) * (0.5 + 0.5 * satellite.light)
    }

    SpaceSphere {
        radius: satellite.pose.radius || 0
        light: 0.5 + 0.5 * satellite.light
        glow: 1
    }

    SpaceGlow {
        radius: (satellite.pose.radius || 0) * 0.92
        strength: 0.7 + 0.3 * satellite.chosen
        plateau: 0.42
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
