import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Wayland

ShellRoot {
    id: root

    property var sessions: []
    property var shownSessions: []
    property int selectedIndex: 0
    property int maxShown: 30
    property string mode: "select"
    property string filterBuffer: ""
    property string activeFilter: ""
    property string createBuffer: ""
    property string renameBuffer: ""
    property string renameTarget: ""
    property string leaderBuffer: ""
    property string preferredSession: ""
    property bool dataLoading: true
    readonly property string editCursor: "\u2588"
    readonly property var selectedSession: shownSessions.length > 0 ? shownSessions[selectedIndex] : null
    readonly property bool filterMode: mode === "filter"
    readonly property bool createMode: mode === "create"
    readonly property bool renameMode: mode === "rename"
    readonly property bool leaderMode: mode === "leader"
    readonly property color accent: Qt.color("#ffffff")

    function matchesFilter(session, query) {
        if (query.length === 0)
            return true;

        return String(session.name || "").toLowerCase().indexOf(query) === 0;
    }

    function rebuildMatches() {
        const query = root.activeFilter.trim().toLowerCase();
        let matches = root.sessions;
        if (query.length > 0)
            matches = root.sessions.filter((session) => {
            return root.matchesFilter(session, query);
        });

        root.shownSessions = matches.slice(0, root.maxShown);
        if (root.preferredSession.length > 0 && root.maxShown > 0) {
            const preferredIndex = matches.findIndex((session) => {
                return (session.name || "") === root.preferredSession;
            });
            if (preferredIndex >= root.maxShown) {
                const preferred = matches[preferredIndex];
                root.shownSessions = matches.slice(0, root.maxShown - 1).concat([preferred]);
            }
        }
        if (root.selectedIndex >= root.shownSessions.length)
            root.selectedIndex = Math.max(0, root.shownSessions.length - 1);

    }

    function angleFor(index) {
        if (root.shownSessions.length === 0)
            return -90;

        return (360 * index) / root.shownSessions.length - 90;
    }

    function moveSelection(delta) {
        if (root.shownSessions.length === 0)
            return ;

        root.selectedIndex = (root.selectedIndex + delta + root.shownSessions.length) % root.shownSessions.length;
    }

    function loadData(showLoading) {
        if (dataLoader.running)
            return ;

        if (showLoading || root.sessions.length === 0)
            root.dataLoading = true;
        dataLoader.running = true;
    }

    function selectPreferred(name) {
        if (!name)
            return ;

        for (let i = 0; i < root.shownSessions.length; i++) {
            if ((root.shownSessions[i].name || "") === name) {
                root.selectedIndex = i;
                return ;
            }
        }
    }

    function shellQuote(value) {
        return "'" + String(value).replace(/'/g, "'\"'\"'") + "'";
    }

    function isEscapeKey(event) {
        return event.key === Qt.Key_Escape || (event.key === Qt.Key_BracketLeft && (event.modifiers & Qt.ControlModifier));
    }

    function writeResult(action, name) {
        const cleanedName = String(name || "").trim();
        if (cleanedName.length === 0)
            return ;

        writer.command = ["bash", "-lc", "printf '%s\\t%s\\n' " + root.shellQuote(action) + " " + root.shellQuote(cleanedName) + " > \"$TM_QUICKSHELL_PICKER_OUT\""];
        writer.running = true;
    }

    function beginFilter() {
        root.mode = "filter";
        root.filterBuffer = "";
    }

    function commitFilter() {
        root.activeFilter = root.filterBuffer.trim().toLowerCase();
        root.mode = "select";
        root.rebuildMatches();
    }

    function clearFilter() {
        root.activeFilter = "";
        root.filterBuffer = "";
        root.mode = "select";
        root.rebuildMatches();
    }

    function beginCreate() {
        root.mode = "create";
        root.createBuffer = "";
    }

    function beginRename() {
        if (!root.selectedSession)
            return ;

        root.mode = "rename";
        root.renameTarget = root.selectedSession.name || "";
        root.renameBuffer = root.renameTarget;
    }

    function beginLeader() {
        root.mode = "leader";
        root.leaderBuffer = "";
    }

    function finishCreate() {
        const cleanedName = root.createBuffer.trim();
        if (cleanedName.length === 0)
            return ;

        root.preferredSession = cleanedName;
        creator.command = ["bash", "-lc", "\"$TM_QUICKSHELL_PICKER_COMMAND\" __picker-create " + root.shellQuote(cleanedName) + " \"$TM_QUICKSHELL_PICKER_DATA\""];
        creator.running = true;
    }

    function finishDelete() {
        if (!root.selectedSession)
            return ;

        const name = root.selectedSession.name || "";
        if (name.length === 0)
            return ;

        deleter.command = ["bash", "-lc", "\"$TM_QUICKSHELL_PICKER_COMMAND\" __picker-delete " + root.shellQuote(name) + " \"$TM_QUICKSHELL_PICKER_DATA\""];
        deleter.running = true;
    }

    function finishRename() {
        const cleanedName = root.renameBuffer.trim();
        if (root.renameTarget.length === 0 || cleanedName.length === 0)
            return ;

        root.preferredSession = cleanedName;
        renamer.command = ["bash", "-lc", "\"$TM_QUICKSHELL_PICKER_COMMAND\" __picker-rename " + root.shellQuote(root.renameTarget) + " " + root.shellQuote(cleanedName) + " \"$TM_QUICKSHELL_PICKER_DATA\""];
        renamer.running = true;
    }

    function finishSelect() {
        if (root.selectedSession)
            root.writeResult("select", root.selectedSession.name || "");

    }

    function appendFilterText(event) {
        const text = event.text || "";
        if (text.length === 1 && /^[ -~]$/.test(text))
            root.filterBuffer += text;

    }

    function appendCreateText(event) {
        const text = event.text || "";
        if (text.length === 1 && /^[ -~]$/.test(text))
            root.createBuffer += text;

    }

    function appendRenameText(event) {
        const text = event.text || "";
        if (text.length === 1 && /^[ -~]$/.test(text))
            root.renameBuffer += text;

    }

    function handleFilterKey(event) {
        if (root.isEscapeKey(event)) {
            root.mode = "select";
            root.filterBuffer = "";
            event.accepted = true;
        } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || (event.key === Qt.Key_M && (event.modifiers & Qt.ControlModifier))) {
            root.commitFilter();
            event.accepted = true;
        } else if (event.key === Qt.Key_Backspace) {
            root.filterBuffer = root.filterBuffer.slice(0, -1);
            event.accepted = true;
        } else if (event.key === Qt.Key_U && (event.modifiers & Qt.ControlModifier)) {
            root.filterBuffer = "";
            event.accepted = true;
        } else {
            root.appendFilterText(event);
            event.accepted = true;
        }
    }

    function handleCreateKey(event) {
        if (root.isEscapeKey(event)) {
            root.mode = "select";
            root.createBuffer = "";
            event.accepted = true;
        } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || (event.key === Qt.Key_M && (event.modifiers & Qt.ControlModifier))) {
            root.finishCreate();
            event.accepted = true;
        } else if (event.key === Qt.Key_Backspace) {
            root.createBuffer = root.createBuffer.slice(0, -1);
            event.accepted = true;
        } else if (event.key === Qt.Key_U && (event.modifiers & Qt.ControlModifier)) {
            root.createBuffer = "";
            event.accepted = true;
        } else {
            root.appendCreateText(event);
            event.accepted = true;
        }
    }

    function handleRenameKey(event) {
        if (root.isEscapeKey(event)) {
            root.mode = "select";
            root.renameBuffer = "";
            root.renameTarget = "";
            event.accepted = true;
        } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || (event.key === Qt.Key_M && (event.modifiers & Qt.ControlModifier))) {
            root.finishRename();
            event.accepted = true;
        } else if (event.key === Qt.Key_Backspace) {
            root.renameBuffer = root.renameBuffer.slice(0, -1);
            event.accepted = true;
        } else if (event.key === Qt.Key_U && (event.modifiers & Qt.ControlModifier)) {
            root.renameBuffer = "";
            event.accepted = true;
        } else {
            root.appendRenameText(event);
            event.accepted = true;
        }
    }

    function handleLeaderKey(event) {
        if (root.isEscapeKey(event)) {
            root.mode = "select";
            root.leaderBuffer = "";
            event.accepted = true;
        } else if (root.leaderBuffer === "" && event.key === Qt.Key_R) {
            root.leaderBuffer = "r";
            event.accepted = true;
        } else if (root.leaderBuffer === "r" && event.key === Qt.Key_N) {
            root.leaderBuffer = "";
            root.beginRename();
            event.accepted = true;
        } else {
            root.mode = "select";
            root.leaderBuffer = "";
            event.accepted = true;
        }
    }

    function detailFor(session) {
        if (!session)
            return "";

        const parts = [];
        parts.push((session.windows || 0) + "W");
        parts.push((session.attached || 0) > 0 ? "ATTACHED" : "FREE");
        if ((session.agent_working || 0) > 0)
            parts.push("AI WORKING");
        else if ((session.agent_total || 0) > 0)
            parts.push("AI IDLE");
        return parts.join("  ");
    }

    function centerLine() {
        if (root.createMode)
            return root.createBuffer.length > 0 ? root.createBuffer + root.editCursor : root.editCursor + "NAME";

        if (root.renameMode)
            return root.renameBuffer.length > 0 ? root.renameBuffer + root.editCursor : root.editCursor + "NAME";

        if (root.filterMode)
            return "/" + root.filterBuffer + root.editCursor;

        if (root.leaderMode)
            return "," + root.leaderBuffer;

        if (root.activeFilter.length > 0)
            return "/" + root.activeFilter;

        return root.selectedSession ? root.detailFor(root.selectedSession) : (root.dataLoading ? "loading saved sessions" : "");
    }

    function handleKey(event) {
        if (root.filterMode) {
            root.handleFilterKey(event);
        } else if (root.createMode) {
            root.handleCreateKey(event);
        } else if (root.renameMode) {
            root.handleRenameKey(event);
        } else if (root.leaderMode) {
            root.handleLeaderKey(event);
        } else if (root.isEscapeKey(event)) {
            if (root.activeFilter.length > 0)
                root.clearFilter();
            else
                Qt.quit();
            event.accepted = true;
        } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || (event.key === Qt.Key_M && (event.modifiers & Qt.ControlModifier))) {
            root.finishSelect();
            event.accepted = true;
        } else if (event.key === Qt.Key_Space) {
            root.finishSelect();
            event.accepted = true;
        } else if (event.key === Qt.Key_Left || event.key === Qt.Key_Up) {
            root.moveSelection(-1);
            event.accepted = true;
        } else if (event.key === Qt.Key_Right || event.key === Qt.Key_Down || event.key === Qt.Key_Tab) {
            root.moveSelection(1);
            event.accepted = true;
        } else if (event.key === Qt.Key_H || event.key === Qt.Key_K) {
            root.moveSelection(-1);
            event.accepted = true;
        } else if (event.key === Qt.Key_L || event.key === Qt.Key_J) {
            root.moveSelection(1);
            event.accepted = true;
        } else if (event.key === Qt.Key_Slash) {
            root.beginFilter();
            event.accepted = true;
        } else if (event.key === Qt.Key_N) {
            root.beginCreate();
            event.accepted = true;
        } else if (event.key === Qt.Key_X) {
            root.finishDelete();
            event.accepted = true;
        } else if (event.key === Qt.Key_Comma) {
            root.beginLeader();
            event.accepted = true;
        } else if (event.key >= Qt.Key_1 && event.key <= Qt.Key_9) {
            const index = event.key - Qt.Key_1;
            if (index < root.shownSessions.length) {
                root.selectedIndex = index;
                root.writeResult("select", root.shownSessions[index].name || "");
            }
            event.accepted = true;
        }
    }

    Component.onCompleted: root.loadData(true)

    Timer {
        interval: 1200
        repeat: true
        running: true
        onTriggered: {
            if (!root.filterMode && !root.createMode && !root.renameMode && !root.leaderMode)
                root.loadData(false);
        }
    }

    Process {
        id: dataLoader

        running: false
        command: ["bash", "-lc", "\"$TM_QUICKSHELL_PICKER_COMMAND\" __picker-payload"]

        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    const parsed = JSON.parse(this.text);
                    const preferred = parsed.preferred || root.preferredSession;
                    root.preferredSession = preferred;
                    root.sessions = parsed.sessions || [];
                    root.rebuildMatches();
                    root.selectPreferred(preferred);
                    root.preferredSession = "";
                } catch (error) {
                    console.warn("Unable to parse tesseract sessions:", error);
                    root.sessions = [];
                    root.rebuildMatches();
                }
                root.dataLoading = false;
            }
        }

    }

    Process {
        id: creator

        running: false
        onExited: {
            root.mode = "select";
            root.createBuffer = "";
            root.activeFilter = "";
            root.filterBuffer = "";
            root.loadData();
        }
    }

    Process {
        id: deleter

        running: false
        onExited: {
            root.mode = "select";
            root.createBuffer = "";
            root.renameBuffer = "";
            root.renameTarget = "";
            root.leaderBuffer = "";
            root.loadData();
        }
    }

    Process {
        id: renamer

        running: false
        onExited: {
            root.mode = "select";
            root.renameBuffer = "";
            root.renameTarget = "";
            root.leaderBuffer = "";
            root.activeFilter = "";
            root.filterBuffer = "";
            root.loadData();
        }
    }

    Process {
        id: writer

        running: false
        onExited: Qt.quit()
    }

    PanelWindow {
        id: panel

        readonly property real shortSide: Math.min(width, height)
        readonly property int visibleCount: root.shownSessions.length
        readonly property real orbitRadius: visibleCount > 18 ? Math.max(190, Math.min(shortSide * 0.38, 360)) : Math.max(154, Math.min(shortSide * 0.31, 270))
        readonly property real tileSize: visibleCount > 24 ? Math.max(36, Math.min(shortSide * 0.055, 48)) : (visibleCount > 18 ? Math.max(44, Math.min(shortSide * 0.07, 58)) : (visibleCount > 12 ? Math.max(56, Math.min(shortSide * 0.09, 70)) : Math.max(70, Math.min(shortSide * 0.12, 92))))
        readonly property bool showTileLabels: visibleCount <= 12
        readonly property real stageSize: Math.max(440, orbitRadius * 2 + tileSize * 2 + 84)

        exclusiveZone: 0
        color: "transparent"
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.keyboardFocus: WlrKeyboardFocus.Exclusive
        WlrLayershell.namespace: "tesseract-session-picker"

        anchors {
            top: true
            bottom: true
            left: true
            right: true
        }

        Item {
            anchors.fill: parent
            focus: true
            Keys.onPressed: (event) => {
                return root.handleKey(event);
            }
            Component.onCompleted: forceActiveFocus()
        }

        MouseArea {
            anchors.fill: parent
            onClicked: Qt.quit()
        }

        Item {
            id: stage

            width: panel.stageSize
            height: panel.stageSize
            anchors.centerIn: parent

            RingField {
                anchors.fill: parent
                orbitRadius: panel.orbitRadius
                accent: root.accent
                focusAngle: root.angleFor(root.selectedIndex)
            }

            Column {
                id: centerStack

                anchors.centerIn: parent
                width: Math.min(420, parent.width * 0.58)
                spacing: 12

                Text {
                    width: parent.width
                    horizontalAlignment: Text.AlignHCenter
                    text: root.createMode ? "NEW SESSION" : (root.renameMode ? "RENAME" : (root.filterMode ? "FILTER" : (root.leaderMode ? "COMMAND" : (root.selectedSession ? String(root.selectedSession.name).toUpperCase() : (root.dataLoading ? "RESTORING" : "NO SESSIONS")))))
                    color: root.accent
                    elide: Text.ElideRight
                    font.family: "JetBrainsMono Nerd Font"
                    font.pixelSize: 27
                    font.weight: Font.Bold
                }

                Rectangle {
                    anchors.horizontalCenter: parent.horizontalCenter
                    width: 88
                    height: 2
                    color: root.accent
                    opacity: 0.88
                }

                Text {
                    width: parent.width
                    horizontalAlignment: Text.AlignHCenter
                    text: root.centerLine()
                    color: root.filterMode || root.createMode || root.renameMode || root.leaderMode ? "#ffffff" : "#707070"
                    elide: Text.ElideMiddle
                    font.family: "JetBrainsMono Nerd Font"
                    font.pixelSize: root.filterMode || root.createMode || root.renameMode || root.leaderMode ? 17 : 10
                    font.weight: root.filterMode || root.createMode || root.renameMode || root.leaderMode ? Font.Bold : Font.Normal
                }

            }

            Repeater {
                model: root.shownSessions

                OrbitTile {
                    required property var modelData
                    required property int index
                    readonly property real angle: root.angleFor(index) * Math.PI / 180

                    appName: modelData.name || ""
                    iconName: ""
                    accent: root.accent
                    selected: index === root.selectedIndex
                    agentTotal: modelData.agent_total || 0
                    agentWorking: modelData.agent_working || 0
                    agentUnseen: modelData.agent_unseen || 0
                    showLabel: panel.showTileLabels
                    tileSize: panel.tileSize
                    x: stage.width / 2 - width / 2 + Math.cos(angle) * panel.orbitRadius
                    y: stage.height / 2 - height / 2 + Math.sin(angle) * panel.orbitRadius

                    MouseArea {
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onEntered: root.selectedIndex = parent.index
                        onClicked: root.writeResult("select", parent.modelData.name || "")
                    }

                    Behavior on x {
                        NumberAnimation {
                            duration: 180
                            easing.type: Easing.OutCubic
                        }

                    }

                    Behavior on y {
                        NumberAnimation {
                            duration: 180
                            easing.type: Easing.OutCubic
                        }

                    }

                }

            }

        }

        Row {
            spacing: 8

            anchors {
                bottom: parent.bottom
                horizontalCenter: parent.horizontalCenter
                bottomMargin: 30
            }

            Repeater {
                model: root.shownSessions.length

                Rectangle {
                    required property int index

                    width: index === root.selectedIndex ? 32 : 12
                    height: 4
                    color: index === root.selectedIndex ? root.accent : Qt.rgba(1, 1, 1, 0.22)

                    Behavior on width {
                        NumberAnimation {
                            duration: 160
                            easing.type: Easing.OutCubic
                        }

                    }

                    Behavior on color {
                        ColorAnimation {
                            duration: 160
                        }

                    }

                }

            }

        }

    }

}
