module Dashboard.Msgs exposing (DragOver(..), Msg(..))

import Concourse
import Concourse.Cli as Cli
import Dashboard.Models as Models
import Keyboard
import Time
import Window


type Msg
    = ClockTick Time.Time
    | AutoRefresh Time.Time
    | ShowFooter
    | KeyPressed Keyboard.KeyCode
    | KeyDowns Keyboard.KeyCode
    | DragStart Concourse.PipelineIdentifier
    | DragOver DragOver
    | DragEnd
    | Tooltip String String
    | TooltipHd String String
    | TogglePipelinePaused Models.Pipeline
    | PipelineButtonHover (Maybe Models.Pipeline)
    | CliHover (Maybe Cli.Cli)
    | TopCliHover (Maybe Cli.Cli)
    | ResizeScreen Window.Size
    | LogIn
    | LogOut
    | FilterMsg String
    | FocusMsg
    | BlurMsg
    | SelectMsg Int
    | ToggleUserMenu
    | ShowSearchInput


type DragOver
    = Before Concourse.PipelineIdentifier
    | End
