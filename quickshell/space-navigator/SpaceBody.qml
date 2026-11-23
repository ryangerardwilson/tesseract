import QtQuick

// SpaceBody is one workspace as a glass sphere with its number on it: the
// nucleus at the centre, large and glowing, or a small body on the orbit,
// dimmer the further round the orbit it turns.
Item {
    id: body

    required property var space
    required property var pose
    required property bool active

    x: pose.x
    y: pose.y
    z: -pose.depth

    SpaceSphere {
        opacity: body.pose.seen
        visible: opacity > 0.004
        radius: body.pose.radius
        light: body.pose.light
        glow: body.pose.hold

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
