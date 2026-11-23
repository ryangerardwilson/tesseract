import QtQuick
import QtQuick.Shapes

// SpaceRing is a thin ring, or half of one, already projected to stage
// points: a hairline over a faint wide glow, and under both a soft band of
// light that shows as `shimmer` rises. It is white unless given a tint.
Shape {
    id: ring

    required property SpaceCamera lens
    property var points: []
    property real light: 1
    property real weight: 1
    // How bright the soft band under the ring is, from 0 for none.
    property real shimmer: 0
    property color color: "white"
    readonly property real band: (0.75 + 0.25 * weight) * lens.unit

    anchors.fill: parent
    preferredRendererType: Shape.CurveRenderer
    visible: light > 0.004 && points.length > 1

    ShapePath {
        strokeColor: Qt.rgba(ring.color.r, ring.color.g, ring.color.b, 0.012 * ring.shimmer * ring.light)
        strokeWidth: 26 * ring.band
        fillColor: "transparent"
        capStyle: ShapePath.FlatCap
        joinStyle: ShapePath.RoundJoin

        PathPolyline {
            path: ring.points
        }

    }

    ShapePath {
        strokeColor: Qt.rgba(ring.color.r, ring.color.g, ring.color.b, 0.016 * ring.shimmer * ring.light)
        strokeWidth: 13 * ring.band
        fillColor: "transparent"
        capStyle: ShapePath.FlatCap
        joinStyle: ShapePath.RoundJoin

        PathPolyline {
            path: ring.points
        }

    }

    ShapePath {
        strokeColor: Qt.rgba(ring.color.r, ring.color.g, ring.color.b, (0.05 + 0.03 * ring.shimmer) * ring.light)
        strokeWidth: 5 * ring.weight * ring.lens.unit
        fillColor: "transparent"
        capStyle: ShapePath.FlatCap
        joinStyle: ShapePath.RoundJoin

        PathPolyline {
            path: ring.points
        }

    }

    ShapePath {
        strokeColor: Qt.rgba(ring.color.r, ring.color.g, ring.color.b, Math.min(1, 0.55 * ring.light))
        strokeWidth: ring.weight * ring.lens.unit
        fillColor: "transparent"
        capStyle: ShapePath.FlatCap
        joinStyle: ShapePath.RoundJoin

        PathPolyline {
            path: ring.points
        }

    }

}
