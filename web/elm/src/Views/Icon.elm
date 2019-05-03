module Views.Icon exposing (icon, iconWithTooltip)

import Html exposing (Html)
import Views.Views as Views exposing (style)


icon :
    { sizePx : Int, image : String }
    -> List (Html.Attribute msg)
    -> Views.View msg
icon { sizePx, image } attrs =
    iconWithTooltip { sizePx = sizePx, image = image } attrs []


iconWithTooltip :
    { sizePx : Int, image : String }
    -> List (Html.Attribute msg)
    -> List (Views.View msg)
    -> Views.View msg
iconWithTooltip { sizePx, image } attrs tooltipContent =
    Views.div Views.Unidentified
        [ style "background-image" ("url(/public/images/" ++ image ++ ")")
        , style "height" (String.fromInt sizePx ++ "px")
        , style "width" (String.fromInt sizePx ++ "px")
        , style "background-position" "50% 50%"
        , style "background-repeat" "no-repeat"
        ]
        attrs
        tooltipContent
