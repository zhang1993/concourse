module Pipeline.Styles exposing
    ( cliIcon
    , groupItem
    , groupsBar
    , pauseToggle
    , pinBadge
    , pinDropdownCursor
    , pinHoverHighlight
    , pinIcon
    , pinIconContainer
    , pinIconDropdown
    , pinText
    )

import Colors
import Concourse.Cli as Cli
import Html
import Html.Attributes exposing (style)
import Views.Views as Views


groupsBar : List (Html.Attribute msg)
groupsBar =
    [ style "background-color" Colors.groupsBarBackground
    , style "color" Colors.dashboardText
    , style "display" "flex"
    , style "flex-flow" "row wrap"
    , style "padding" "5px"
    ]


groupItem : { selected : Bool, hovered : Bool } -> List (Html.Attribute msg)
groupItem { selected, hovered } =
    [ style "font-size" "14px"
    , style "background" Colors.groupBackground
    , style "margin" "5px"
    , style "padding" "10px"
    ]
        ++ (if selected then
                [ style "opacity" "1"
                , style "border" <| "1px solid " ++ Colors.groupBorderSelected
                ]

            else if hovered then
                [ style "opacity" "0.6"
                , style "border" <| "1px solid " ++ Colors.groupBorderHovered
                ]

            else
                [ style "opacity" "0.6"
                , style "border" <| "1px solid " ++ Colors.groupBorderUnselected
                ]
           )


pinHoverHighlight : List Views.Style
pinHoverHighlight =
    [ Views.style "border-width" "5px"
    , Views.style "border-style" "solid"
    , Views.style "border-color" <| "transparent transparent " ++ Colors.white ++ " transparent"
    , Views.style "position" "absolute"
    , Views.style "top" "100%"
    , Views.style "right" "50%"
    , Views.style "margin-right" "-5px"
    , Views.style "margin-top" "-10px"
    ]


pinText : List Views.Style
pinText =
    [ Views.style "font-weight" "700" ]


pinDropdownCursor : List Views.Style
pinDropdownCursor =
    [ Views.style "cursor" "pointer" ]


pinIconDropdown : List Views.Style
pinIconDropdown =
    [ Views.style "background-color" Colors.white
    , Views.style "color" Colors.pinIconHover
    , Views.style "position" "absolute"
    , Views.style "top" "100%"
    , Views.style "right" "0"
    , Views.style "white-space" "nowrap"
    , Views.style "list-style-type" "none"
    , Views.style "padding" "10px"
    , Views.style "margin-top" "0"
    , Views.style "z-index" "1"
    ]


pinIcon : List Views.Style
pinIcon =
    [ Views.style "background-image" "url(/public/images/pin-ic-white.svg)"
    , Views.style "width" "40px"
    , Views.style "height" "40px"
    , Views.style "background-repeat" "no-repeat"
    , Views.style "background-position" "50% 50%"
    , Views.style "position" "relative"
    ]


pinBadge : List Views.Style
pinBadge =
    [ Views.style "background-color" Colors.pinned
    , Views.style "border-radius" "50%"
    , Views.style "width" "15px"
    , Views.style "height" "15px"
    , Views.style "position" "absolute"
    , Views.style "top" "3px"
    , Views.style "right" "3px"
    , Views.style "display" "flex"
    , Views.style "align-items" "center"
    , Views.style "justify-content" "center"
    ]


pinIconContainer : Bool -> List Views.Style
pinIconContainer showBackground =
    [ Views.style "margin-right" "15px"
    , Views.style "top" "10px"
    , Views.style "position" "relative"
    , Views.style "height" "40px"
    , Views.style "display" "flex"
    , Views.style "max-width" "20%"
    ]
        ++ (if showBackground then
                [ Views.style "background-color" Colors.pinHighlight
                , Views.style "border-radius" "50%"
                ]

            else
                []
           )


pauseToggle : Bool -> List (Html.Attribute msg)
pauseToggle isPaused =
    [ style "border-left" <|
        if isPaused then
            "1px solid rgba(255, 255, 255, 0.5)"

        else
            "1px solid #3d3c3c"
    ]


cliIcon : Cli.Cli -> List (Html.Attribute msg)
cliIcon cli =
    [ style "width" "12px"
    , style "height" "12px"
    , style "background-image" <| Cli.iconUrl cli
    , style "background-repeat" "no-repeat"
    , style "background-position" "50% 50%"
    , style "background-size" "contain"
    , style "display" "inline-block"
    ]
