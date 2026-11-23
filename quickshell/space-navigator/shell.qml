import QtQuick
import Quickshell
import Quickshell.Wayland

// The `tesseract open space` overlay: one full-screen panel that hosts the
// space navigator, gives it the keys, and quits when it is done or
// dismissed. SpaceNavigator owns the data, the keys, and the commands.
ShellRoot {
    id: root

    PanelWindow {
        id: panel

        exclusiveZone: 0
        color: "transparent"
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.keyboardFocus: WlrKeyboardFocus.Exclusive
        WlrLayershell.namespace: "tesseract-space-navigator"

        anchors {
            top: true
            bottom: true
            left: true
            right: true
        }

        SpaceNavigator {
            id: navigator

            anchors.fill: parent
            command: Quickshell.env("TESSERACT_SPACE_COMMAND") || "tesseract"
            onHideRequested: {
                panel.WlrLayershell.keyboardFocus = WlrKeyboardFocus.None;
                panel.visible = false;
            }
            onFinished: Qt.quit()
            onCloseRequested: Qt.quit()
            Component.onCompleted: navigator.open()
        }

        Item {
            anchors.fill: parent
            focus: true
            Keys.onPressed: (event) => {
                return navigator.handleKey(event);
            }
            Component.onCompleted: forceActiveFocus()
        }

        MouseArea {
            anchors.fill: parent
            onClicked: Qt.quit()
        }

    }

}
