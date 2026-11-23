import QtQuick
import QtQuick.Shapes

// SpaceGlow is a soft disc of light centred on its own position, brightest
// in the middle and gone at `radius`. It is white unless given a tint.
Item {
    id: glow

    property real radius: 100
    property real strength: 0.1
    // How far out, as a fraction of the radius, the light holds full strength.
    property real plateau: 0
    property color color: "white"

    Shape {
        x: -100
        y: -100
        width: 200
        height: 200
        scale: glow.radius / 100
        preferredRendererType: Shape.CurveRenderer
        visible: glow.strength > 0.002 && glow.radius > 0.5

        ShapePath {
            strokeColor: "transparent"

            fillGradient: RadialGradient {
                centerX: 100
                centerY: 100
                centerRadius: 100
                focalX: 100
                focalY: 100

                GradientStop {
                    position: 0
                    color: Qt.rgba(glow.color.r, glow.color.g, glow.color.b, glow.strength)
                }

                GradientStop {
                    position: glow.plateau
                    color: Qt.rgba(glow.color.r, glow.color.g, glow.color.b, glow.strength)
                }

                GradientStop {
                    position: glow.plateau + (1 - glow.plateau) * 0.3
                    color: Qt.rgba(glow.color.r, glow.color.g, glow.color.b, glow.strength * 0.42)
                }

                GradientStop {
                    position: glow.plateau + (1 - glow.plateau) * 0.65
                    color: Qt.rgba(glow.color.r, glow.color.g, glow.color.b, glow.strength * 0.1)
                }

                GradientStop {
                    position: 1
                    color: Qt.rgba(glow.color.r, glow.color.g, glow.color.b, 0)
                }

            }

            PathAngleArc {
                centerX: 100
                centerY: 100
                radiusX: 100
                radiusY: 100
                startAngle: 0
                sweepAngle: 360
            }

        }

    }

}
