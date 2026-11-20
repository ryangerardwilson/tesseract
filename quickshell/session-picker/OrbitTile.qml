import QtQuick
import Quickshell

Item {
    id: tile

    property string appName: ""
    property string iconName: ""
    property color accent: "#ffffff"
    property bool selected: false
    property int agentTotal: 0
    property int agentWorking: 0
    property int agentUnseen: 0
    readonly property bool showUnseen: agentWorking === 0 && agentUnseen > 0
    property bool showLabel: true
    property real tileSize: 88

    function iconSource(name) {
        if (!name)
            return "";

        if (name.indexOf("/") === 0)
            return "file://" + name;

        return Quickshell.iconPath(name, true);
    }

    width: tileSize + 28
    height: tileSize + 42
    scale: selected ? 1.1 : 1
    opacity: selected ? 1 : 0.74

    Rectangle {
        id: halo

        anchors.horizontalCenter: parent.horizontalCenter
        anchors.top: parent.top
        width: tile.tileSize + 16
        height: width
        radius: width / 2
        color: "transparent"
        border.width: tile.selected ? 2 : 1
        border.color: tile.selected ? tile.accent : Qt.rgba(1, 1, 1, 0.2)
        opacity: tile.selected ? 0.78 : 0.44
    }

    Item {
        id: aiOrbit

        anchors.centerIn: halo
        width: halo.width + 12
        height: width
        visible: tile.agentWorking > 0
        opacity: tile.selected ? 1 : 0.72

        layer.enabled: visible
        layer.smooth: true

        Rectangle {
            width: Math.max(15, tile.tileSize * 0.24)
            height: Math.max(2, tile.tileSize * 0.03)
            radius: height / 2
            anchors.horizontalCenter: parent.horizontalCenter
            y: 1
            color: tile.accent
            opacity: 0.98
        }

        Rectangle {
            width: Math.max(10, tile.tileSize * 0.15)
            height: Math.max(2, tile.tileSize * 0.025)
            radius: height / 2
            anchors.horizontalCenter: parent.horizontalCenter
            y: parent.height - height - 1
            color: tile.accent
            opacity: 0.68
        }

        Rectangle {
            width: Math.max(2, tile.tileSize * 0.025)
            height: Math.max(9, tile.tileSize * 0.14)
            radius: width / 2
            anchors.verticalCenter: parent.verticalCenter
            x: parent.width - width - 1
            color: tile.accent
            opacity: 0.5
        }

        Rectangle {
            width: Math.max(2, tile.tileSize * 0.025)
            height: Math.max(9, tile.tileSize * 0.14)
            radius: width / 2
            anchors.verticalCenter: parent.verticalCenter
            x: 1
            color: tile.accent
            opacity: 0.5
        }

        RotationAnimator on rotation {
            from: 0
            to: 360
            duration: 1400
            loops: Animation.Infinite
            running: aiOrbit.visible
            easing.type: Easing.Linear
        }

    }

    Item {
        id: unseenRing

        readonly property real tick: Math.max(14, tile.tileSize * 0.18)
        readonly property real hairline: Math.max(2.5, tile.tileSize * 0.032)
        property real breath: 0

        anchors.centerIn: halo
        width: halo.width + 22
        height: width
        visible: tile.showUnseen
        opacity: 0.55 + 0.45 * breath

        Rectangle {
            width: unseenRing.tick
            height: unseenRing.hairline
            radius: height / 2
            anchors.horizontalCenter: parent.horizontalCenter
            y: 1
            color: tile.accent
        }

        Rectangle {
            width: unseenRing.hairline
            height: unseenRing.tick
            radius: width / 2
            anchors.verticalCenter: parent.verticalCenter
            x: 1
            color: tile.accent
        }

        Rectangle {
            width: unseenRing.hairline
            height: unseenRing.tick
            radius: width / 2
            anchors.verticalCenter: parent.verticalCenter
            x: parent.width - width - 1
            color: tile.accent
        }

        SequentialAnimation on breath {
            running: unseenRing.visible
            loops: Animation.Infinite

            NumberAnimation {
                from: 0
                to: 1
                duration: 800
                easing.type: Easing.InOutSine
            }

            NumberAnimation {
                from: 1
                to: 0
                duration: 800
                easing.type: Easing.InOutSine
            }

        }

    }

    Rectangle {
        id: button

        anchors.centerIn: halo
        width: tile.tileSize
        height: width
        radius: width / 2
        color: tile.selected ? Qt.rgba(1, 1, 1, 0.08) : Qt.rgba(0, 0, 0, 0.48)
        border.width: 1
        border.color: tile.selected ? tile.accent : Qt.rgba(1, 1, 1, 0.22)
    }

    Image {
        id: icon

        anchors.centerIn: button
        width: button.width * 0.48
        height: width
        smooth: true
        asynchronous: true
        fillMode: Image.PreserveAspectFit
        source: tile.iconSource(tile.iconName)
        visible: status === Image.Ready
    }

    Text {
        anchors.centerIn: button
        visible: !icon.visible
        text: tile.appName.length > 0 ? tile.appName.charAt(0).toUpperCase() : "?"
        color: tile.selected ? tile.accent : "#ffffff"
        font.family: "JetBrainsMono Nerd Font"
        font.pixelSize: tile.tileSize * 0.36
        font.weight: Font.Bold
    }

    Text {
        anchors.horizontalCenter: button.horizontalCenter
        anchors.top: button.bottom
        anchors.topMargin: 10
        visible: tile.showLabel
        width: parent.width
        horizontalAlignment: Text.AlignHCenter
        text: tile.appName.toUpperCase()
        color: tile.selected ? "#ffffff" : "#8d8d8d"
        elide: Text.ElideRight
        font.family: "JetBrainsMono Nerd Font"
        font.pixelSize: 10
        font.weight: Font.Bold
    }

    Behavior on scale {
        NumberAnimation {
            duration: 160
            easing.type: Easing.OutCubic
        }

    }

    Behavior on opacity {
        NumberAnimation {
            duration: 160
        }

    }

}
